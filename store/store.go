package store

import (
	"context"

	"github.com/projecteru2/agent/types"
)

type Store interface {
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	// SetNodeStatus reports the node alive under core's ttl.
	SetNodeStatus(ctx context.Context, ttl int64) error
	ListRunningWorkloadIDs(ctx context.Context) ([]string, error)
	SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error
	// WorkloadExists reports whether core still owns the workload.
	WorkloadExists(ctx context.Context, ID string) (bool, error)
	GetIdentifier(ctx context.Context) string
}
