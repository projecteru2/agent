package workload

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAttach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := newMockWorkloadManager(t)
	go func() {
		for {
			log := <-manager.logBroadcaster.logC
			// see: runtime.FromTemplate
			switch log.Type {
			case "stdout":
				assert.Equal(t, log.Data, "stdout")
			case "stderr":
				assert.Equal(t, log.Data, "stderr")
			}
		}
	}()

	manager.attach(ctx, "Rei")
	time.Sleep(2 * time.Second)
}
