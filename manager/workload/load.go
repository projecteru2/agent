package workload

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
)

func (m *Manager) initWorkloadStatus(ctx context.Context) error {
	log.Info("[initWorkloadStatus] Load workloads")
	workloadIDs, err := m.runtimeClient.ListWorkloadIDs(ctx, m.getBaseFilter())
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	for _, wid := range workloadIDs {
		log.Debugf("[initWorkloadStatus] detect workload %s", wid)
		wg.Add(1)
		go func(ID string) {
			defer wg.Done()
			workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
			if err != nil {
				log.Errorf("[initWorkloadStatus] get workload status failed %v", err)
				return
			}

			if workloadStatus.Running {
				log.Debugf("[initWorkloadStatus] workload %s is running", workloadStatus.ID)
				go m.attach(ctx, ID)
			}

			// no health check here
			if err := m.setWorkloadStatus(ctx, workloadStatus); err != nil {
				log.Errorf("[initWorkloadStatus] update workload %v status failed %v", ID, err)
			}
		}(wid)
	}
	wg.Wait()
	return nil
}
