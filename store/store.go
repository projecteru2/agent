package store

import (
	"context"

	"github.com/projecteru2/agent/types"
	coretypes "github.com/projecteru2/core/types"
)

// Store indicate store
type Store interface {
	GetNode(nodename string) (*coretypes.Node, error)
	UpdateNode(node *coretypes.Node) error

	SetNodeStatus(context.Context, int64) error
	SetContainerStatus(context.Context, *types.Container, *coretypes.Node) error

	GetCoreIdentifier() string
}
