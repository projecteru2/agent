package store

import "github.com/projecteru2/agent/types"

type Store interface {
	Crash() error
	RegisterNode(node *types.Node) error

	UpdateStats(node *types.Node) error
	UpdateContainer(container *types.Container) error

	GetContainer(cid string) (*types.Container, error)
	RemoveContainer(cid string) error

	GetAllContainers() ([]string, error)
}
