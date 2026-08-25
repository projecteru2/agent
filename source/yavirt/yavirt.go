package yavirt

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/projecteru2/libyavirt/client"
	yavirttypes "github.com/projecteru2/libyavirt/types"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

var _ source.Source = (*Yavirt)(nil)

type Yavirt struct {
	client     client.Client
	config     *types.Config
	filter     map[string]string
	skipRegexp []*regexp.Regexp
}

func New(ctx context.Context, config *types.Config) (*Yavirt, error) {
	y := &Yavirt{
		config: config,
		filter: map[string]string{cluster.ERUMark: "1"},
	}

	var err error
	if y.client, err = utils.MakeYavirtClient(config); err != nil {
		return nil, err
	}

	for _, expr := range y.config.Yavirt.SkipGuestReportRegexps {
		reg, err := regexp.Compile(expr)
		if err != nil {
			log.WithFunc("yavirt.New").Errorf(ctx, err, "failed to compile regexp %v", expr)
			return nil, err
		}
		y.skipRegexp = append(y.skipRegexp, reg)
	}

	return y, nil
}

func (y *Yavirt) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("yavirt.List")

	var ids []string
	var err error
	utils.WithTimeout(ctx, y.config.GlobalConnectionTimeout, func(ctx context.Context) {
		ids, err = y.client.GetGuestIDList(ctx, yavirttypes.GetGuestIDListReq{Filters: y.filter})
	})
	if err != nil && !strings.Contains(err.Error(), "key not exists") {
		logger.Error(ctx, err, "failed to get workload ids")
		return nil, err
	}

	workloads := make([]*source.Workload, 0, len(ids))
	for _, ID := range ids {
		w, err := y.Get(ctx, ID)
		if err != nil {
			logger.WithField("ID", ID).Debugf(ctx, "skipping workload: %v", err)
			continue
		}
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (y *Yavirt) Get(ctx context.Context, ID string) (*source.Workload, error) {
	if y.needSkip(ID) {
		return nil, common.ErrInvalidVM
	}
	logger := log.WithFunc("yavirt.Get").WithField("ID", ID)

	var guest yavirttypes.Guest
	var err error
	utils.WithTimeout(ctx, y.config.GlobalConnectionTimeout, func(ctx context.Context) {
		guest, err = y.client.GetGuest(ctx, ID)
	})
	if err != nil {
		logger.Error(ctx, err, "failed to detect workload")
		return nil, err
	}

	if _, ok := guest.Labels[cluster.ERUMark]; !ok {
		return nil, common.ErrInvalidVM
	}
	if y.config.CheckOnlyMine && y.config.HostName != guest.Hostname {
		logger.Debugf(ctx, "guest's hostname is %s instead of %s", guest.Hostname, y.config.HostName)
		return nil, common.ErrInvalidVM
	}

	w := &source.Workload{
		ID: guest.ID,
		Meta: source.Meta{
			Nodename:    y.config.HostName,
			Labels:      guest.Labels,
			HealthCheck: coreutils.DecodeMetaInLabel(ctx, guest.Labels).HealthCheck,
			Networks:    guest.Networks,
		},
		Running: guest.Running,
	}
	if len(guest.IPs) > 0 {
		w.LocalIP = guest.IPs[0]
	}
	return w, nil
}

func (y *Yavirt) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error)
	yaEventChan, yaErrChan := y.client.Events(ctx, y.filter)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for {
			select {
			case msg := <-yaEventChan:
				eventChan <- &types.WorkloadEventMessage{
					ID:       msg.ID,
					Type:     msg.Type,
					Action:   msg.Action,
					TimeNano: msg.TimeNano,
				}
			case err := <-yaErrChan:
				errChan <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, errChan
}

func (y *Yavirt) Alive(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, y.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = y.client.Info(ctx)
	})
	if err != nil {
		log.WithFunc("yavirt.Alive").Error(ctx, err, "connect to yavirt daemon failed")
		return false
	}
	return true
}

func (y *Yavirt) needSkip(ID string) bool {
	return slices.ContainsFunc(y.skipRegexp, func(reg *regexp.Regexp) bool { return reg.MatchString(ID) })
}
