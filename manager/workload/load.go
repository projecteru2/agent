package workload

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/log"
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
				log.Error(ctx, err, "[initWorkloadStatus] Failed to load workloads, will retry")
				continue
			}
			return workloadIDs, nil
		}
	}
}

func (m *Manager) initWorkloadStatus(ctx context.Context) error {
	log.Info(ctx, "[initWorkloadStatus] Load workloads")
	workloadIDs, err := m.listWorkloadIDsWithRetry(ctx, m.getBaseFilter())
	if err != nil {
		log.Error(ctx, err, "[initWorkloadStatus] Failed to load workloads")
		return err
	}

	wg := &sync.WaitGroup{}
	for _, workloadID := range workloadIDs {
		log.Debugf(ctx, "[initWorkloadStatus] detect workload %s", workloadID)
		wg.Add(1)
		ID := workloadID
		_ = utils.Pool.Submit(func() {
			defer wg.Done()
			workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
			if err != nil {
				log.Errorf(ctx, err, "[initWorkloadStatus] get workload %v status failed", ID)
				return
			}

			if workloadStatus.Running {
				log.Debugf(ctx, "[initWorkloadStatus] workload %s is running", workloadStatus.ID)
				_ = utils.Pool.Submit(func() { m.attach(ctx, ID) })
			}

			if err := m.setWorkloadStatus(ctx, workloadStatus); err != nil {
				log.Errorf(ctx, err, "[initWorkloadStatus] update workload %v status failed", ID)
			}
		})
	}
	wg.Wait()
	return nil
}
