package workload

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

func (m *Manager) listWorkloadIDsWithRetry(ctx context.Context, filter map[string]string) ([]string, error) {
	var workloadIDs []string
	var err error
	ticker := time.NewTicker(m.config.GlobalConnectionTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			workloadIDs, err = m.runtimeClient.ListWorkloadIDs(ctx, filter)
			if err != nil {
				log.Errorf("[initWorkloadStatus] Failed to load workloads: %v, will retry", err)
				continue
			}
			return workloadIDs, nil
		}
	}
}

func (m *Manager) initWorkloadStatus(ctx context.Context) error {
	log.Info("[initWorkloadStatus] Load workloads")
	workloadIDs, err := m.listWorkloadIDsWithRetry(ctx, m.getBaseFilter())
	if err != nil {
		log.Errorf("[initWorkloadStatus] Failed to load workloads: %v", err)
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
				log.Errorf("[initWorkloadStatus] get workload %v status failed %v", ID, err)
				return
			}

			if workloadStatus.Running {
				log.Debugf("[initWorkloadStatus] workload %s is running", workloadStatus.ID)
				go m.attach(ctx, ID)
			}

			if err := m.setWorkloadStatus(ctx, workloadStatus); err != nil {
				log.Errorf("[initWorkloadStatus] update workload %v status failed %v", ID, err)
			}
		}(wid)
	}
	wg.Wait()
	return nil
}
