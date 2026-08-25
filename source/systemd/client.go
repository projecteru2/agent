package systemd

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
)

var (
	once       sync.Once
	systemd    *Systemd
	systemdErr error
)

// GetClient returns the process-wide systemd source, creating it on first call.
func GetClient(ctx context.Context, config *types.Config) (*Systemd, error) {
	once.Do(func() { systemd, systemdErr = New(ctx, config) })
	return systemd, systemdErr
}
