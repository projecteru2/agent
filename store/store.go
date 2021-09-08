package store

import (
	"context"

	"github.com/projecteru2/agent/types"
)

// Store wrapper of remote calls
type Store interface {
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	UpdateNode(ctx context.Context, node *types.Node) error
	SetNodeStatus(ctx context.Context, ttl int64) error
	GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error)
	SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus, ttl int64) error
	SetNode(ctx context.Context, node string, status bool) error
	GetIdentifier(ctx context.Context) string
	NodeStatusStream(ctx context.Context) (<-chan *types.NodeStatus, <-chan error)
	ListPodNodes(ctx context.Context, all bool, podname string, labels map[string]string) ([]*types.Node, error)
}
