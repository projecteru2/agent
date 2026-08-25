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
			select {
			case <-ctx.Done():
				return
			case log := <-manager.logBroadcaster.logC:
				switch log.Type {
				case "stdout":
					assert.Equal(t, log.Data, "stdout")
				case "stderr":
					assert.Equal(t, log.Data, "stderr")
				}
			}
		}
	}()

	rei, err := manager.source.Get(ctx, "Rei")
	assert.Nil(t, err)
	go manager.attach(ctx, rei)
	go manager.attach(ctx, rei)
	time.Sleep(2 * time.Second)
}
