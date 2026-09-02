package workload

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/projecteru2/core/log"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const sweepFanout = 64

func (m *Manager) healthCheck(ctx context.Context) {
	tick := time.NewTicker(time.Duration(m.config.HealthCheck.Interval) * time.Second)
	defer tick.Stop()

	var sweeping atomic.Bool
	for {
		select {
		case <-tick.C:
			if !sweeping.CompareAndSwap(false, true) {
				continue
			}
			go func() {
				defer sweeping.Store(false)
				m.checkAllWorkloads(ctx)
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) checkAllWorkloads(ctx context.Context) {
	logger := log.WithFunc("workload.checkAllWorkloads")
	logger.Debug(ctx, "health check begin")
	runningIDs, err := m.store.ListRunningWorkloadIDs(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to list running workloads from core")
	}

	workloads, err := m.source.List(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to list workloads")
		return
	}

	listed := make(map[string]struct{}, len(workloads))
	var g errgroup.Group
	g.SetLimit(sweepFanout)
	for _, w := range workloads {
		listed[w.ID] = struct{}{}
		g.Go(func() error { m.checkOneWorkload(ctx, w); return nil })
	}
	for _, ID := range runningIDs {
		if _, ok := listed[ID]; !ok {
			g.Go(func() error { m.reconcile(ctx, ID); return nil })
		}
	}
	_ = g.Wait()

	for _, ID := range runningIDs {
		listed[ID] = struct{}{}
	}
	for ID := range m.localTaskIDs() {
		if _, ok := listed[ID]; ok {
			continue
		}
		if _, err := m.source.Get(ctx, ID); err != nil {
			m.stop(ID)
		}
	}
}

func (m *Manager) localTaskIDs() map[string]struct{} {
	IDs := map[string]struct{}{}
	m.collectMutex.Lock()
	for ID := range m.collecting {
		IDs[ID] = struct{}{}
	}
	m.collectMutex.Unlock()
	m.logMutex.RLock()
	for _, target := range m.logTargets {
		IDs[target.workload.ID] = struct{}{}
	}
	m.logMutex.RUnlock()
	return IDs
}

func (m *Manager) reconcile(ctx context.Context, ID string) {
	if w, err := m.source.Get(ctx, ID); err == nil {
		m.checkOneWorkload(ctx, w)
		return
	}
	m.handleWorkloadDie(ctx, &types.WorkloadEventMessage{ID: ID})
}

func (m *Manager) checkOneWorkload(ctx context.Context, w *source.Workload) bool {
	logger := log.WithFunc("workload.checkOneWorkload").WithField("ID", w.ID)
	if w.Running {
		m.start(ctx, w)
	} else {
		m.stop(w.ID)
	}

	status, err := m.workloadStatus(ctx, w)
	if errors.Is(err, common.ErrGetLockFailed) {
		logger.Debug(ctx, "another probe of this workload is running, it reports the status")
		return false
	}
	if err != nil {
		logger.Error(ctx, err, "failed to get status of workload")
		return false
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
		return m.store.SetWorkloadStatus(ctx, status)
	})
}
