package workload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAttach(t *testing.T) {
	ctx := t.Context()
	manager := newMockWorkloadManager(t)
	go func() {
		for {
			log := <-manager.logBroadcaster.logC
			switch log.Type {
			case "stdout":
				assert.Equal(t, log.Data, "stdout")
			case "stderr":
				assert.Equal(t, log.Data, "stderr")
			}
		}
	}()

	go manager.attach(ctx, "Rei")
	go manager.attach(ctx, "Rei")
	time.Sleep(2 * time.Second)
}
