package containerd

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
)

var (
	once      sync.Once
	client    *Containerd
	clientErr error
)

// GetClient returns the process-wide containerd source, creating it on first call.
func GetClient(ctx context.Context, config *types.Config, nodeIP, storeIdentifier string) (*Containerd, error) {
	once.Do(func() { client, clientErr = New(ctx, config, nodeIP, storeIdentifier) })
	return client, clientErr
}
