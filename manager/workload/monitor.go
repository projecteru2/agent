package workload

import (
	"context"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"

	log "github.com/sirupsen/logrus"
)

var eventHandler = NewEventHandler()

func (m *Manager) initMonitor(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventHandler.Handle(common.StatusStart, m.handleWorkloadStart)
	eventHandler.Handle(common.StatusDie, m.handleWorkloadDie)

	f := m.getFilter(map[string]string{})
	eventChan, errChan := m.runtimeClient.Events(ctx, f)
	return eventChan, errChan
}

func (m *Manager) monitor(ctx context.Context, eventChan <-chan *types.WorkloadEventMessage) {
	log.Info("[monitor] Status watch start")
	eventHandler.Watch(ctx, eventChan)
}

func (m *Manager) handleWorkloadStart(ctx context.Context, event *types.WorkloadEventMessage) {
	log.Debugf("[handleWorkloadStart] workload %s start", coreutils.ShortID(event.ID))
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
		go m.checkOneWorkloadWithBackoffRetry(ctx, event.ID)
	}
}

func (m *Manager) handleWorkloadDie(ctx context.Context, event *types.WorkloadEventMessage) {
	log.Debugf("[handleWorkloadDie] container %s die", coreutils.ShortID(event.ID))
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, event.ID, true)
	if err != nil {
		log.Errorf("[handleWorkloadDie] faild to get workload %v status, err: %v", event.ID, err)
		return
	}

	if err := m.store.SetWorkloadStatus(ctx, workloadStatus, m.config.GetHealthCheckStatusTTL()); err != nil {
		log.Errorf("[handleWorkloadDie] update deploy status failed %v", err)
	}
}
