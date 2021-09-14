package runtime

import (
	"context"
	"io"

	"github.com/projecteru2/agent/types"
)

// Runtime provides runtime-related functions
type Runtime interface {
	AttachWorkload(ctx context.Context, ID string) (io.Reader, io.Reader, error)
	CollectWorkloadMetrics(ctx context.Context, ID string)
	ListWorkloadIDs(ctx context.Context, filters map[string]string) ([]string, error)
	Events(ctx context.Context, filters map[string]string) (<-chan *types.WorkloadEventMessage, <-chan error)
	GetStatus(ctx context.Context, ID string, checkHealth bool) (*types.WorkloadStatus, error)
	GetWorkloadName(ctx context.Context, ID string) (string, error)
	LogFieldsExtra(ctx context.Context, ID string) (map[string]string, error)
	IsDaemonRunning(ctx context.Context) bool
	Name() string
}
