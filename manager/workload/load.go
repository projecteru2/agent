package workload

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

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

	var wg sync.WaitGroup
	for _, w := range workloads {
		logger.Debugf(ctx, "detect workload %s", w.ID)
		wg.Go(func() {
			if w.Running {
				logger.Debugf(ctx, "workload %s is running", w.ID)
				m.start(ctx, w)
			}
			m.checkOneWorkload(ctx, w)
		})
	}
	wg.Wait()
	return nil
}
