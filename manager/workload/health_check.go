package workload

import (
	"context"
	"encoding/json"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
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
	workloads, err := m.source.List(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to list workloads")
		return
	}

	for _, w := range workloads {
		go m.checkOneWorkload(ctx, w)
	}
}

func (m *Manager) checkOneWorkload(ctx context.Context, w *source.Workload) bool {
	logger := log.WithFunc("workload.checkOneWorkload").WithField("ID", w.ID)
	status, err := m.workloadStatus(ctx, w)
	if err != nil {
		logger.Error(ctx, err, "failed to get status of workload")
		return false
	}
	if status.Running {
		m.start(ctx, w)
	}

	if err = m.setWorkloadStatus(ctx, status); err != nil {
		logger.Error(ctx, err, "update workload status failed")
	}
	return status.Healthy
}

func (m *Manager) workloadStatus(ctx context.Context, w *source.Workload) (*types.WorkloadStatus, error) {
	labels, err := json.Marshal(w.Meta.Labels)
	if err != nil {
		return nil, err
	}

	status := &types.WorkloadStatus{
		ID:         w.ID,
		Running:    w.Running,
		Healthy:    w.Running && w.Meta.HealthCheck == nil,
		Networks:   w.Meta.Networks,
		Extension:  labels,
		Appname:    w.Meta.Appname,
		Nodename:   m.config.HostName,
		Entrypoint: w.Meta.Entrypoint,
	}
	if !w.Running || w.Meta.HealthCheck == nil {
		return status, nil
	}

	free, acquired := m.cas.Acquire(w.ID)
	if !acquired {
		return nil, common.ErrGetLockFailed
	}
	defer free()
	status.Healthy = collector.Probe(ctx, w, time.Duration(m.config.HealthCheck.Timeout)*time.Second)
	return status, nil
}

func (m *Manager) setWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error {
	return utils.BackoffRetry(ctx, 3, func() error {
		return m.store.SetWorkloadStatus(ctx, status, m.config.GetHealthCheckStatusTTL())
	})
}
