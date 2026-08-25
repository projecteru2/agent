package workload

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

var eventHandler = NewEventHandler()

func (m *Manager) initMonitor(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventHandler.Handle(common.StatusStart, m.handleWorkloadStart)
	eventHandler.Handle(common.StatusDie, m.handleWorkloadDie)

	return m.source.Events(ctx)
}

func (m *Manager) watchEvent(ctx context.Context, eventChan <-chan *types.WorkloadEventMessage) {
	log.WithFunc("workload.watchEvent").Info(ctx, "status watch start")
	eventHandler.Watch(ctx, eventChan)
}

func (m *Manager) monitor(ctx context.Context) {
	logger := log.WithFunc("workload.monitor")
	for {
		eventChan, errChan := m.initMonitor(ctx)
		go m.watchEvent(ctx, eventChan)
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

	retryTask := utils.NewRetryTask(ctx, utils.GetMaxAttemptsByTTL(m.config.GetHealthCheckStatusTTL()), func() error {
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
		if err := retryTask.Run(ctx); err != nil {
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
		logger.Error(ctx, err, "failed to get workload")
		return
	}

	if w.Running {
		m.start(ctx, w)
	}

	status, err := m.workloadStatus(ctx, w)
	if err != nil {
		logger.Error(ctx, err, "failed to get workload status")
		return
	}
	if !status.Healthy {
		m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
		return
	}
	if err := m.store.SetWorkloadStatus(ctx, status, m.config.GetHealthCheckStatusTTL()); err != nil {
		logger.Error(ctx, err, "failed to update workload status")
	}
}

func (m *Manager) handleWorkloadDie(ctx context.Context, event *types.WorkloadEventMessage) {
	logger := log.WithFunc("workload.handleWorkloadDie").WithField("ID", event.ID)
	logger.Debug(ctx, "handling die")
	m.stopCollecting(event.ID)
	m.stopForwarding(event.ID)

	w, err := m.source.Get(ctx, event.ID)
	if err != nil {
		logger.Error(ctx, err, "failed to get workload")
		return
	}
	status, err := m.workloadStatus(ctx, w)
	if err != nil {
		logger.Error(ctx, err, "failed to get workload status")
		return
	}

	if err := m.store.SetWorkloadStatus(ctx, status, m.config.GetHealthCheckStatusTTL()); err != nil {
		logger.Error(ctx, err, "failed to update workload status")
	}
}
