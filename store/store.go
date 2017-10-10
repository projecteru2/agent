package store

import "github.com/projecteru2/core/types"

type Store interface {
	Crash(node *types.Node) error
	GetNode(nodename string) (*types.Node, error)
	UpdateNode(node *types.Node) error

	GetContainer(cid string) (*types.Container, error)
	UpdateContainer(container *types.Container) error
}
