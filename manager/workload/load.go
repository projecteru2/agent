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
				log.WithFunc("listWorkloadIDsWithRetry").Error(ctx, err, "failed to load workloads, will retry")
				continue
			}
			return workloadIDs, nil
		}
	}
}

func (m *Manager) initWorkloadStatus(ctx context.Context) error {
	logger := log.WithFunc("initWorkloadStatus")
	logger.Info(ctx, "load workloads")
	workloadIDs, err := m.listWorkloadIDsWithRetry(ctx, m.getBaseFilter())
	if err != nil {
		logger.Error(ctx, err, "failed to load workloads")
		return err
	}

	wg := &sync.WaitGroup{}
	for _, workloadID := range workloadIDs {
		logger.Debugf(ctx, "detect workload %s", workloadID)
		wg.Add(1)
		ID := workloadID
		_ = utils.Pool.Submit(func() {
			defer wg.Done()
			workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
			if err != nil {
				logger.Errorf(ctx, err, "get workload %v status failed", ID)
				return
			}

			if workloadStatus.Running {
				logger.Debugf(ctx, "workload %s is running", workloadStatus.ID)
				_ = utils.Pool.Submit(func() { m.attach(ctx, ID) })
			}

			if err := m.setWorkloadStatus(ctx, workloadStatus); err != nil {
				logger.Errorf(ctx, err, "update workload %v status failed", ID)
			}
		})
	}
	wg.Wait()
	return nil
}
