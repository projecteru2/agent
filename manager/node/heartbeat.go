package node

import (
	"context"
	"time"

	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

// heartbeat creates a new goroutine to report status every HeartbeatInterval seconds
// By default HeartbeatInterval is 0, will not do heartbeat.
func (m *Manager) heartbeat(ctx context.Context) {
	if m.config.HeartbeatInterval <= 0 {
		return
	}

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

// nodeStatusReport does heartbeat, tells core this node is alive.
// The TTL is set to double of HeartbeatInterval, by default it will be 360s,
// which means if a node is not available, subcriber will notice this after at least 360s.
// HealthCheck.Timeout is used as timeout of requesting core Profile
func (m *Manager) nodeStatusReport(ctx context.Context) {
	log.Debug("[nodeStatusReport] report begins")
	defer log.Debug("[nodeStatusReport] report ends")

	if !m.runtimeClient.IsDaemonRunning(ctx) {
		log.Debugf("[nodeStatusReport] cannot connect to runtime daemon")
		return
	}

	utils.WithTimeout(ctx, time.Duration(m.config.HealthCheck.Timeout)*time.Second, func(ctx context.Context) {
		ttl := int64(m.config.HeartbeatInterval * 2)
		if err := m.store.SetNodeStatus(ctx, ttl); err != nil {
			log.Errorf("[nodeStatusReport] error when set node status: %v", err)
		}
	})
}
