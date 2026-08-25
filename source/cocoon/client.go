package cocoon

import (
	"context"
	"sync"

	"github.com/projecteru2/agent/types"
)

var (
	once      sync.Once
	cocoon    *Cocoon
	cocoonErr error
)

func GetClient(ctx context.Context, config *types.Config) (*Cocoon, error) {
	once.Do(func() { cocoon, cocoonErr = New(ctx, config) })
	return cocoon, cocoonErr
}
