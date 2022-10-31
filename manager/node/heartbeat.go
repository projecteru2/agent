package node

import (
	"context"
	"time"

	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
)

// heartbeat creates a new goroutine to report status every HeartbeatInterval seconds
// By default HeartbeatInterval is 0, will not do heartbeat.
func (m *Manager) heartbeat(ctx context.Context) {
	if m.config.HeartbeatInterval <= 0 {
		return
	}
	_ = utils.Pool.Submit(func() { m.nodeStatusReport(ctx) })

	tick := time.NewTicker(time.Duration(m.config.HeartbeatInterval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			_ = utils.Pool.Submit(func() { m.nodeStatusReport(ctx) })
		case <-ctx.Done():
			return
		}
	}
}

// nodeStatusReport does heartbeat, tells core this node is alive.
// The TTL is set to double of HeartbeatInterval, by default it will be 360s,
// which means if a node is not available, subcriber will notice this after at least 360s.
// HealthCheck.Timeout is used as timeout of requesting core Profile
func (m *Manager) nodeStatusReport(ctx context.Context) {
	log.Debug(ctx, "[nodeStatusReport] report begins")
	defer log.Debug(ctx, "[nodeStatusReport] report ends")

	if !m.runtimeClient.IsDaemonRunning(ctx) {
		log.Warn(ctx, "[nodeStatusReport] cannot connect to runtime daemon")
		return
	}

	ttl := int64(m.config.HeartbeatInterval * 3)

	if err := utils.BackoffRetry(ctx, 3, func() (err error) {
		utils.WithTimeout(ctx, m.config.GlobalConnectionTimeout, func(ctx context.Context) {
			if err = m.store.SetNodeStatus(ctx, ttl); err != nil {
				log.Errorf(ctx, err, "[nodeStatusReport] failed to set node status of %v", m.config.HostName)
			}
		})
		return err
	}); err != nil {
		log.Errorf(ctx, err, "[nodeStatusReport] failed to set node status of %v for 3 times", m.config.HostName)
	}
}
