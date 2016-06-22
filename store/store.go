package store

import "gitlab.ricebook.net/platform/agent/types"

type Store interface {
	Crash() error
	RegisterNode(node *types.Node) error

	UpdateStats(node *types.Node) error
	UpdateContainer(container *types.Container) error

	GetContainer(cid string) (*types.Container, error)
	RemoveContainer(cid string) error
}
