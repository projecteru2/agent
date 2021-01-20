package engine

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// heartbeat creates a new goroutine to report status every NodeStatusInterval seconds
// by default it will be 180s
func (e *Engine) heartbeat(ctx context.Context) {
	tick := time.NewTicker(time.Duration(e.config.HeartbeatInterval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			go e.nodeStatusReport()
		case <-ctx.Done():
			return
		}
	}
}

// nodeStatusReport does heartbeat, tells core this node is alive.
// The TTL is set to double of HeartbeatInterval, by default it will be 360s,
// which means if a node is not available, subcriber will notice this after at least 360s.
// HealthCheck.Timeout is used as timeout of requesting core API
func (e *Engine) nodeStatusReport() {
	log.Debug("[nodeStatusReport] report begins")
	defer log.Debug("[nodeStatusReport] report ends")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.config.HealthCheck.Timeout)*time.Second)
	defer cancel()

	ttl := int64(e.config.HeartbeatInterval * 2)
	if err := e.store.SetNodeStatus(ctx, ttl); err != nil {
		log.Errorf("[nodeStatusReport] error when set node status: %v", err)
	}
}
