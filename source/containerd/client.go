package containerd

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
)

var (
	mutex  sync.Mutex
	client *Containerd
)

// GetClient returns the process-wide containerd source, creating it on the first call that succeeds.
func GetClient(ctx context.Context, config *types.Config, nodeIP, storeIdentifier string) (*Containerd, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if client != nil {
		return client, nil
	}
	source, err := New(ctx, config, nodeIP, storeIdentifier)
	if err != nil {
		return nil, err
	}
	client = source
	return client, nil
}
