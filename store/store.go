package store

import (
	"context"

	"github.com/projecteru2/agent/types"
)

type Store interface {
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	SetNodeStatus(ctx context.Context, ttl int64) error
	ListRunningWorkloadIDs(ctx context.Context) ([]string, error)
	SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error
	WorkloadExists(ctx context.Context, ID string) (bool, error)
	GetIdentifier(ctx context.Context) string
}
