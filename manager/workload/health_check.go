package workload

import (
	"context"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
)

func (m *Manager) healthCheck(ctx context.Context) {
	tick := time.NewTicker(time.Duration(m.config.HealthCheck.Interval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			_ = utils.Pool.Submit(func() { m.checkAllWorkloads(ctx) })
		case <-ctx.Done():
			return
		}
	}
}

// checkAllWorkloads lists stopped workloads too, so a late check cannot resurrect a dead one.
func (m *Manager) checkAllWorkloads(ctx context.Context) {
	logger := log.WithFunc("checkAllWorkloads")
	logger.Debug(ctx, "health check begin")
	workloadIDs, err := m.runtimeClient.ListWorkloadIDs(ctx, m.getBaseFilter())
	if err != nil {
		logger.Error(ctx, err, "error when list all workloads with label \"ERU=1\"")
		return
	}

	for _, workloadID := range workloadIDs {
		ID := workloadID
		_ = utils.Pool.Submit(func() { m.checkOneWorkload(ctx, ID) })
	}
}

func (m *Manager) checkOneWorkload(ctx context.Context, ID string) bool {
	logger := log.WithFunc("checkOneWorkload").WithField("ID", ID)
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
