package workload

import (
	"context"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
)

var eventHandler = NewEventHandler()

func (m *Manager) initMonitor(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventHandler.Handle(common.StatusStart, m.handleWorkloadStart)
	eventHandler.Handle(common.StatusDie, m.handleWorkloadDie)

	eventChan, errChan := m.runtimeClient.Events(ctx, m.getBaseFilter())
	return eventChan, errChan
}

func (m *Manager) watchEvent(ctx context.Context, eventChan <-chan *types.WorkloadEventMessage) {
	log.WithFunc("watchEvent").Info(ctx, "status watch start")
	eventHandler.Watch(ctx, eventChan)
}

// monitor with retry
func (m *Manager) monitor(ctx context.Context) {
	logger := log.WithFunc("monitor")
	for {
		eventChan, errChan := m.initMonitor(ctx)
		_ = utils.Pool.Submit(func() { m.watchEvent(ctx, eventChan) })
		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled, stop monitoring")
			return
		case err := <-errChan:
			logger.Error(ctx, err, "received an err, will retry")
			time.Sleep(m.config.GlobalConnectionTimeout)
		}
	}
}

// 检查一个workload，允许重试
func (m *Manager) checkOneWorkloadWithBackoffRetry(ctx context.Context, ID string) {
	logger := log.WithFunc("checkOneWorkloadWithBackoffRetry").WithField("ID", ID)
	logger.Debug(ctx, "check workload")

	m.checkWorkloadMutex.Lock()
	defer m.checkWorkloadMutex.Unlock()

	if retryTask, ok := m.startingWorkloads.Get(ID); ok {
		retryTask.Stop(ctx)
	}

	retryTask := utils.NewRetryTask(ctx, utils.GetMaxAttemptsByTTL(m.config.GetHealthCheckStatusTTL()), func() error {
		if !m.checkOneWorkload(ctx, ID) {
			// 这个err就是用来判断要不要继续的，不用打在日志里
			return common.ErrWorkloadUnhealthy
		}
		return nil
	})
	m.startingWorkloads.Set(ID, retryTask)
	_ = utils.Pool.Submit(func() {
		if err := retryTask.Run(ctx); err != nil {
			logger.Debug(ctx, "workload still not healthy")
		}
	})
}

func (m *Manager) handleWorkloadStart(ctx context.Context, event *types.WorkloadEventMessage) {
	logger := log.WithFunc("handleWorkloadStart").WithField("ID", event.ID)
	logger.Debug(ctx, "workload start")
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, event.ID, true)
	if err != nil {
		logger.Error(ctx, err, "faild to get workload status")
		return
	}

	if workloadStatus.Running {
		_ = utils.Pool.Submit(func() { m.attach(ctx, event.ID) })
	}

	if workloadStatus.Healthy {
		if err := m.store.SetWorkloadStatus(ctx, workloadStatus, m.config.GetHealthCheckStatusTTL()); err != nil {
			logger.Error(ctx, err, "update deploy status failed")
		}
	} else {
		m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
	}
}

func (m *Manager) handleWorkloadDie(ctx context.Context, event *types.WorkloadEventMessage) {
	logger := log.WithFunc("handleWorkloadDie").WithField("ID", event.ID)
	logger.Debug(ctx, "container die")
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, event.ID, true)
	if err != nil {
		logger.Error(ctx, err, "faild to get workload status")
		return
	}

	if err := m.store.SetWorkloadStatus(ctx, workloadStatus, m.config.GetHealthCheckStatusTTL()); err != nil {
		logger.Error(ctx, err, "update deploy status failed")
	}
}
