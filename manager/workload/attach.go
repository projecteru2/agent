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
	logger := log.WithFunc("attach").WithField("ID", ID)
	logger.Debug(ctx, "attaching workload")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	transfer := m.forwards.Get(ID, 0)
	if transfer == "" {
		transfer = logs.Discard
	}
	writer, err := logs.NewWriter(ctx, transfer, m.config.Log.Stdout)
	if err != nil {
		logger.Errorf(ctx, err, "create log forward %s failed", transfer)
		return
	}

	// get app info
	workloadName, err := m.runtimeClient.GetWorkloadName(ctx, ID)
	if err != nil {
		if err != common.ErrNotImplemented {
			logger.Error(ctx, err, "failed to get workload name")
		} else {
			logger.Debug(ctx, "should ignore this workload")
		}
		return
	}

	name, entryPoint, ident, err := utils.GetAppInfo(workloadName)
	if err != nil {
		logger.Errorf(ctx, err, "invalid workload name %s", workloadName)
		return
	}

	// attach workload
	outr, errr, err := m.runtimeClient.AttachWorkload(ctx, ID)
	if err != nil {
		logger.Errorf(ctx, err, "failed to attach workload %s", workloadName)
		return
	}
	logger.Infof(ctx, "attach %s workload success", workloadName)

	// attach metrics
	_ = utils.Pool.Submit(func() { m.runtimeClient.CollectWorkloadMetrics(ctx, ID) })

	extra, err := m.runtimeClient.LogFieldsExtra(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to get log fields extra")
	}

	wg := &sync.WaitGroup{}
	pump := func(typ string, source io.Reader) {
		defer wg.Done()
		logger.Debugf(ctx, "attach pump %s %s start", workloadName, typ)
		defer logger.Debugf(ctx, "attach pump %s %s finished", workloadName, typ)

		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					logger.Errorf(ctx, err, "attach pump %s %s failed", workloadName, typ)
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
				logger.Errorf(ctx, err, "%s workload %s write failed", workloadName, entryPoint)
			}
		}
	}
	wg.Add(2)
	defer wg.Wait()
	_ = utils.Pool.Submit(func() { pump("stdout", outr) })
	_ = utils.Pool.Submit(func() { pump("stderr", errr) })
}
