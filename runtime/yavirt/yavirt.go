package yavirt

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/libyavirt/client"
	yavirttypes "github.com/projecteru2/libyavirt/types"

	"github.com/projecteru2/core/log"
)

// Yavirt .
type Yavirt struct {
	client     client.Client
	config     *types.Config
	skipRegexp []*regexp.Regexp
	cas        *utils.GroupCAS
}

// New returns a wrapper of yavirt client
func New(config *types.Config) (*Yavirt, error) {
	y := &Yavirt{
		config: config,
		cas:    utils.NewGroupCAS(),
	}

	var err error
	if y.client, err = utils.MakeYavirtClient(config); err != nil {
		return nil, err
	}

	for _, expr := range y.config.Yavirt.SkipGuestReportRegexps {
		reg, err := regexp.Compile(expr)
		if err != nil {
			log.WithFunc("runtime.yavirt.New").Errorf(nil, err, "failed to compile regexp %v", expr) //nolint
			return nil, err
		}
		y.skipRegexp = append(y.skipRegexp, reg)
	}

	return y, nil
}

// needSkip checks if a workload should be skipped
func (y *Yavirt) needSkip(ID string) bool {
	for _, reg := range y.skipRegexp {
		if reg.MatchString(ID) {
			return true
		}
	}
	return false
}

// detectWorkload detects a workload by ID
func (y *Yavirt) detectWorkload(ctx context.Context, ID string) (*Guest, error) {
	if y.needSkip(ID) {
		return nil, common.ErrInvaildVM
	}
	logger := log.WithFunc("detectWorkload").WithField("ID", ID)

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
		return nil, common.ErrInvaildVM
	}

	if y.config.CheckOnlyMine && y.config.HostName != guest.Hostname {
		logger.Debugf(ctx, "guest's hostname is %s instead of %s", guest.Hostname, y.config.HostName)
		return nil, common.ErrInvaildVM
	}

	return &Guest{
		ID:            guest.ID,
		Status:        guest.Status,
		TransitStatus: guest.TransitStatus,
		CreateTime:    guest.CreateTime,
		TransitTime:   guest.TransitTime,
		UpdateTime:    guest.UpdateTime,
		CPU:           guest.CPU,
		Mem:           guest.Mem,
		Storage:       guest.Storage,
		ImageID:       guest.ImageID,
		ImageName:     guest.ImageName,
		ImageUser:     guest.ImageUser,
		Networks:      guest.Networks,
		Labels:        guest.Labels,
		IPs:           guest.IPs,
		Hostname:      guest.Hostname,
		Running:       guest.Running,
		once:          sync.Once{},
	}, nil
}

// AttachWorkload not implemented yet
func (y *Yavirt) AttachWorkload(ctx context.Context, ID string) (io.Reader, io.Reader, error) {
	return nil, nil, common.ErrNotImplemented
}

// CollectWorkloadMetrics no need yet
func (y *Yavirt) CollectWorkloadMetrics(ctx context.Context, ID string) {}

// ListWorkloadIDs lists workload IDs filtered by given condition
func (y *Yavirt) ListWorkloadIDs(ctx context.Context, filters map[string]string) (ids []string, err error) {
	utils.WithTimeout(ctx, y.config.GlobalConnectionTimeout, func(ctx context.Context) {
		ids, err = y.client.GetGuestIDList(ctx, yavirttypes.GetGuestIDListReq{Filters: filters})
	})
	if err != nil && !strings.Contains(err.Error(), "key not exists") {
		log.WithFunc("ListWorkloadIDs").Error(ctx, err, "failed to get workload ids")
		return nil, err
	}
	return ids, nil
}

// Events returns the events of workloads' changes
func (y *Yavirt) Events(ctx context.Context, filters map[string]string) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error)
	yaEventChan, yaErrChan := y.client.Events(ctx, filters)

	_ = utils.Pool.Submit(func() {
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
	})

	return eventChan, errChan
}

// GetStatus checks workload's status first, then returns workload status
func (y *Yavirt) GetStatus(ctx context.Context, ID string, checkHealth bool) (*types.WorkloadStatus, error) {
	logger := log.WithFunc("GetStatus").WithField("ID", ID)
	guest, err := y.detectWorkload(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to get guest status")
		return nil, err
	}

	bytes, err := json.Marshal(guest.Labels)
	if err != nil {
		logger.Error(ctx, err, "failed to marshal labels")
		return nil, err
	}

	status := &types.WorkloadStatus{
		ID:        guest.ID,
		Running:   guest.Running,
		Healthy:   guest.Running && guest.HealthCheck == nil,
		Networks:  guest.Networks,
		Extension: bytes,
		Nodename:  y.config.HostName,
	}

	if checkHealth && guest.Running {
		free, acquired := y.cas.Acquire(guest.ID)
		if !acquired {
			return nil, common.ErrGetLockFailed
		}
		defer free()
		status.Healthy = guest.CheckHealth(ctx, time.Duration(y.config.HealthCheck.Timeout)*time.Second)
	}

	return status, nil
}

// GetWorkloadName not implemented yet
func (y *Yavirt) GetWorkloadName(ctx context.Context, ID string) (string, error) {
	return "", common.ErrNotImplemented
}

// LogFieldsExtra .
func (y *Yavirt) LogFieldsExtra(ctx context.Context, ID string) (map[string]string, error) {
	return map[string]string{}, nil
}

// IsDaemonRunning returns if the runtime daemon is running.
func (y *Yavirt) IsDaemonRunning(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, y.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = y.client.Info(ctx)
	})
	if err != nil {
		log.WithFunc("IsDaemonRunning").Error(ctx, err, "connect to yavirt daemon failed")
		return false
	}
	return true
}

// Name returns the name of runtime
func (y *Yavirt) Name() string {
	return "yavirt"
}
