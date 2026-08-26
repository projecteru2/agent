package node

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/utils"
)

const (
	ttlHeartbeats  = 3
	reportAttempts = 3
)

func (m *Manager) heartbeat(ctx context.Context) {
	go m.nodeStatusReport(ctx)

	tick := time.NewTicker(time.Duration(m.config.HeartbeatInterval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			go m.nodeStatusReport(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) nodeStatusReport(ctx context.Context) {
	logger := log.WithFunc("node.nodeStatusReport").WithField("hostname", m.config.HostName)
	logger.Debug(ctx, "report begins")
	defer logger.Debug(ctx, "report ends")

	if !m.source.Alive(ctx) {
		logger.Warn(ctx, "cannot connect to runtime daemon")
		return
	}

	// the ttl outlives the interval so one lost report cannot expire the node
	ttl := int64(m.config.HeartbeatInterval * ttlHeartbeats)

	if err := utils.BackoffRetry(ctx, reportAttempts, func() error {
		err := m.setNodeStatus(ctx, ttl)
		if err != nil {
			logger.Error(ctx, err, "failed to set node status")
		}
		return err
	}); err != nil {
		logger.Errorf(ctx, err, "failed to set node status after %d attempts", reportAttempts)
	}
}
