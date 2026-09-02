package workload

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/agent/source"
)

func (m *Manager) listWithRetry(ctx context.Context) ([]*source.Workload, error) {
	ticker := time.NewTicker(m.config.GlobalConnectionTimeout)
	defer ticker.Stop()
	for {
		workloads, err := m.source.List(ctx)
		if err == nil {
			return workloads, nil
		}
		log.WithFunc("workload.listWithRetry").Error(ctx, err, "failed to load workloads, will retry")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) initWorkloadStatus(ctx context.Context) error {
	logger := log.WithFunc("workload.initWorkloadStatus")
	logger.Info(ctx, "load workloads")
	workloads, err := m.listWithRetry(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to load workloads")
		return err
	}

	var g errgroup.Group
	g.SetLimit(sweepFanout)
	for _, w := range workloads {
		logger.Debugf(ctx, "detect workload %s", w.ID)
		g.Go(func() error { m.checkOneWorkload(ctx, w); return nil })
	}
	return g.Wait()
}
