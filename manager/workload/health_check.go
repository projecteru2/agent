package workload

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

func (m *Manager) healthCheck(ctx context.Context) {
	tick := time.NewTicker(time.Duration(m.config.HealthCheck.Interval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			go m.checkAllWorkloads(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// checkAllWorkloads lists stopped workloads too, so a late check cannot resurrect a dead one.
func (m *Manager) checkAllWorkloads(ctx context.Context) {
	logger := log.WithFunc("workload.checkAllWorkloads")
	logger.Debug(ctx, "health check begin")
	workloadIDs, err := m.runtimeClient.ListWorkloadIDs(ctx, m.baseFilter)
	if err != nil {
		logger.Error(ctx, err, "failed to list workloads")
		return
	}

	for _, ID := range workloadIDs {
		go m.checkOneWorkload(ctx, ID)
	}
}

func (m *Manager) checkOneWorkload(ctx context.Context, ID string) bool {
	logger := log.WithFunc("workload.checkOneWorkload").WithField("ID", ID)
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
	if err != nil {
		logger.Error(ctx, err, "failed to get status of workload")
		return false
	}

	if err = m.setWorkloadStatus(ctx, workloadStatus); err != nil {
		logger.Error(ctx, err, "update workload status failed")
	}
	return workloadStatus.Healthy
}

func (m *Manager) setWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error {
	return utils.BackoffRetry(ctx, 3, func() error {
		return m.store.SetWorkloadStatus(ctx, status, m.config.GetHealthCheckStatusTTL())
	})
}
