package workload

import (
	"context"
	"errors"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

var eventHandler = NewEventHandler()

func (m *Manager) initMonitor(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventHandler.Handle(common.StatusStart, m.handleWorkloadStart)
	eventHandler.Handle(common.StatusDie, m.handleWorkloadDie)

	eventChan, errChan := m.runtimeClient.Events(ctx, m.getBaseFilter())
	return eventChan, errChan
}

func (m *Manager) watchEvent(ctx context.Context, eventChan <-chan *types.WorkloadEventMessage) {
	log.Info("[watchEvent] Status watch start")
	eventHandler.Watch(ctx, eventChan)
}

// monitor with retry
func (m *Manager) monitor(ctx context.Context) {
	for {
		eventChan, errChan := m.initMonitor(ctx)
		go m.watchEvent(ctx, eventChan)
		select {
		case <-ctx.Done():
			log.Infof("[monitor] context canceled, stop monitoring")
			return
		case err := <-errChan:
			log.Errorf("[monitor] received an err: %v, will retry", err)
			time.Sleep(m.config.GlobalConnectionTimeout)
		}
	}
}

// 检查一个workload，允许重试
func (m *Manager) checkOneWorkloadWithBackoffRetry(ctx context.Context, ID string) {
	log.Debugf("[checkOneWorkloadWithBackoffRetry] check workload %s", ID)

	m.checkWorkloadMutex.Lock()
	defer m.checkWorkloadMutex.Unlock()

	if v, ok := m.startingWorkloads.Load(ID); ok {
		retryTask := v.(*utils.RetryTask)
		retryTask.Stop()
	}

	retryTask := utils.NewRetryTask(ctx, utils.GetMaxAttemptsByTTL(m.config.GetHealthCheckStatusTTL()), func() error {
		if !m.checkOneWorkload(ctx, ID) {
			// 这个err就是用来判断要不要继续的，不用打在日志里
			return errors.New("not healthy")
		}
		return nil
	})
	m.startingWorkloads.Store(ID, retryTask)
	go func() {
		err := retryTask.Run()
		if err != nil {
			log.Debugf("[checkOneWorkloadWithBackoffRetry] workload %s still not healthy", ID)
		}
	}()
}

func (m *Manager) handleWorkloadStart(ctx context.Context, event *types.WorkloadEventMessage) {
	log.Debugf("[handleWorkloadStart] workload %s start", event.ID)
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, event.ID, true)
	if err != nil {
		log.Errorf("[handleWorkloadStart] faild to get workload %v status, err: %v", event.ID, err)
		return
	}

	if workloadStatus.Running {
		go m.attach(ctx, event.ID)
	}

	if workloadStatus.Healthy {
		if err := m.store.SetWorkloadStatus(ctx, workloadStatus, m.config.GetHealthCheckStatusTTL()); err != nil {
			log.Errorf("[handleWorkloadStart] update deploy status failed %v", err)
		}
	} else {
		m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
	}
}

func (m *Manager) handleWorkloadDie(ctx context.Context, event *types.WorkloadEventMessage) {
	log.Debugf("[handleWorkloadDie] container %s die", event.ID)
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, event.ID, true)
	if err != nil {
		log.Errorf("[handleWorkloadDie] faild to get workload %v status, err: %v", event.ID, err)
		return
	}

	if err := m.store.SetWorkloadStatus(ctx, workloadStatus, m.config.GetHealthCheckStatusTTL()); err != nil {
		log.Errorf("[handleWorkloadDie] update deploy status failed %v", err)
	}
}
