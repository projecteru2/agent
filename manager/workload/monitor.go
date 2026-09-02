package workload

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const startingCheckAttempts = 5

func (m *Manager) monitor(ctx context.Context) {
	logger := log.WithFunc("workload.monitor")
	for {
		eventChan, errChan := m.source.Events(ctx)
		logger.Info(ctx, "status watch start")
		go m.events.Watch(ctx, eventChan)
		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled, stop monitoring")
			return
		case err, ok := <-errChan:
			if !ok {
				logger.Info(ctx, "event stream closed, stop monitoring")
				return
			}
			logger.Error(ctx, err, "received an err, will retry")
			time.Sleep(m.config.GlobalConnectionTimeout)
		}
	}
}

func (m *Manager) checkOneWorkloadWithBackoffRetry(ctx context.Context, ID string) {
	logger := log.WithFunc("workload.checkOneWorkloadWithBackoffRetry").WithField("ID", ID)
	logger.Debug(ctx, "check workload")

	m.checkWorkloadMutex.Lock()
	defer m.checkWorkloadMutex.Unlock()

	if retryTask, ok := m.startingWorkloads[ID]; ok {
		retryTask.Stop()
	}

	retryTask := utils.NewRetryTask(ctx, startingCheckAttempts, func() error {
		w, err := m.source.Get(ctx, ID)
		if err != nil {
			return err
		}
		if !m.checkOneWorkload(ctx, w) {
			return common.ErrWorkloadUnhealthy
		}
		return nil
	})
	m.startingWorkloads[ID] = retryTask
	go func() {
		if err := retryTask.Run(); err != nil {
			logger.Debug(ctx, "workload still not healthy")
		}
		m.forgetRetryTask(ID, retryTask)
	}()
}

func (m *Manager) forgetRetryTask(ID string, retryTask *utils.RetryTask) {
	m.checkWorkloadMutex.Lock()
	defer m.checkWorkloadMutex.Unlock()
	if m.startingWorkloads[ID] == retryTask {
		delete(m.startingWorkloads, ID)
	}
}

func (m *Manager) handleWorkloadStart(ctx context.Context, event *types.WorkloadEventMessage) {
	logger := log.WithFunc("workload.handleWorkloadStart").WithField("ID", event.ID)
	logger.Debug(ctx, "handling start")
	w, err := m.source.Get(ctx, event.ID)
	if err != nil {
		logger.Error(ctx, err, "failed to get workload, will retry")
		m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
		return
	}

	if !m.checkOneWorkload(ctx, w) {
		m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
	}
}

func (m *Manager) handleWorkloadDie(ctx context.Context, event *types.WorkloadEventMessage) {
	logger := log.WithFunc("workload.handleWorkloadDie").WithField("ID", event.ID)
	logger.Debug(ctx, "handling die")
	forwarded := m.forwardedWorkload(event.ID)
	m.stop(event.ID)

	w, err := m.source.Get(ctx, event.ID)
	if err == nil {
		m.checkOneWorkload(ctx, w)
		if !w.Running {
			m.stop(event.ID)
		}
		return
	}

	if owned, ownErr := m.store.WorkloadExists(ctx, event.ID); ownErr == nil && !owned {
		logger.Debugf(ctx, "no runtime knows it and core has removed it: %v", err)
		m.stop(event.ID)
		return
	}
	logger.Warnf(ctx, "no runtime knows it any more, reporting it gone: %v", err)
	status := &types.WorkloadStatus{ID: event.ID, Nodename: m.config.HostName}
	if forwarded != nil {
		status.Appname = forwarded.Meta.Appname
		status.Entrypoint = forwarded.Meta.Entrypoint
	}
	if err := m.setWorkloadStatus(ctx, status); err != nil {
		logger.Error(ctx, err, "failed to update workload status")
	}
	m.stop(event.ID)
}
