package engine

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// statusReport creates a new goroutine to report status every NodeStatusInterval seconds
// by default it will be 180s
func (e *Engine) statusReport() {
	tick := time.NewTicker(time.Duration(e.config.HealthCheck.NodeStatusInterval) * time.Second)
	// TODO this Stop is never reached
	// fix this in another PR
	defer tick.Stop()
	for range tick.C {
		go e.nodeStatusReport()
	}
}

// nodeStatusReport does heartbeat, tells core this node is alive
// the TTL is set to double of NodeStatusInterval, by default it will be 360s
// which means if a node is not available, subcriber will notice this
// after at least 360s
func (e *Engine) nodeStatusReport() {
	log.Debug("[nodeStatusReport] report begins")
	defer log.Debug("[nodeStatusReport] report ends")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(e.config.HealthCheck.Timeout)*time.Second)
	defer cancel()

	ttl := int64(e.config.HealthCheck.NodeStatusInterval * 2)
	if err := e.store.SetNodeStatus(ctx, ttl); err != nil {
		log.Errorf("[nodeStatusReport] error when set node status: %v", err)
	}
}
