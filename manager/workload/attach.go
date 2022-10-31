package workload

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/logs"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
)

func (m *Manager) attach(ctx context.Context, ID string) {
	log.Debugf(ctx, "[attach] attaching workload %v", ID)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	transfer := m.forwards.Get(ID, 0)
	if transfer == "" {
		transfer = logs.Discard
	}
	writer, err := logs.NewWriter(ctx, transfer, m.config.Log.Stdout)
	if err != nil {
		log.Errorf(ctx, err, "[attach] Create log forward %s failed", transfer)
		return
	}

	// get app info
	workloadName, err := m.runtimeClient.GetWorkloadName(ctx, ID)
	if err != nil {
		if err != common.ErrNotImplemented {
			log.Errorf(ctx, err, "[attach] failed to get workload name, id: %v", ID)
		} else {
			log.Debug(ctx, "[attach] should ignore this workload")
		}
		return
	}

	name, entryPoint, ident, err := utils.GetAppInfo(workloadName)
	if err != nil {
		log.Errorf(ctx, err, "[attach] invalid workload name %s", workloadName)
		return
	}

	// attach workload
	outr, errr, err := m.runtimeClient.AttachWorkload(ctx, ID)
	if err != nil {
		log.Errorf(ctx, err, "[attach] failed to attach workload %s", workloadName)
		return
	}
	log.Infof(ctx, "[attach] attach %s workload %s success", workloadName, ID)

	// attach metrics
	_ = utils.Pool.Submit(func() { m.runtimeClient.CollectWorkloadMetrics(ctx, ID) })

	extra, err := m.runtimeClient.LogFieldsExtra(ctx, ID)
	if err != nil {
		log.Error(ctx, err, "[attach] failed to get log fields extra")
	}

	wg := &sync.WaitGroup{}
	pump := func(typ string, source io.Reader) {
		defer wg.Done()
		log.Debugf(ctx, "[attach] attach pump %s %s %s start", workloadName, ID, typ)
		defer log.Debugf(ctx, "[attach] attach pump %s %s %s finished", workloadName, ID, typ)

		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Errorf(ctx, err, "[attach] attach pump %s %s %s failed, err: %v", workloadName, ID, typ, err)
				}
				return
			}
			data = strings.TrimSuffix(data, "\n")
			data = strings.TrimSuffix(data, "\r")
			l := &types.Log{
				ID:         ID,
				Name:       name,
				Type:       typ,
				EntryPoint: entryPoint,
				Ident:      ident,
				Data:       utils.ReplaceNonUtf8(data),
				Datetime:   time.Now().Format(common.DateTimeFormat),
				Extra:      extra,
			}
			if m.logBroadcaster != nil && m.logBroadcaster.logC != nil {
				m.logBroadcaster.logC <- l
			}
			if err := writer.Write(l); err != nil && !(entryPoint == "agent" && utils.IsDockerized()) {
				log.Errorf(ctx, err, "[attach] %s workload %s_%s write failed", workloadName, entryPoint, ID)
			}
		}
	}
	wg.Add(2)
	defer wg.Wait()
	_ = utils.Pool.Submit(func() { pump("stdout", outr) })
	_ = utils.Pool.Submit(func() { pump("stderr", errr) })
}
