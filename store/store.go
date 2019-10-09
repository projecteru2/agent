package store

import (
	"github.com/projecteru2/agent/types"
	coretypes "github.com/projecteru2/core/types"
)

// Store indicate store
type Store interface {
	GetNode(nodename string) (*coretypes.Node, error)
	UpdateNode(node *coretypes.Node) error

	DeployContainerStats(container *types.Container, node *coretypes.Node) error
}
