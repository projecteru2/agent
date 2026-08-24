package yavirt

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
)

var (
	once      sync.Once
	yavirt    *Yavirt
	yavirtErr error
)

// GetClient returns the process-wide yavirt runtime, creating it on first call.
func GetClient(ctx context.Context, config *types.Config) (*Yavirt, error) {
	once.Do(func() { yavirt, yavirtErr = New(ctx, config) })
	return yavirt, yavirtErr
}
