package store

import "gitlab.ricebook.net/platform/agent/types"

type Store interface {
	Crash() error
	UpdateStats(stats *types.NodeStats) error
	RegisterNode(stats *types.NodeStats) error
	UpdateContainer(stats *types.ContainerStats) error
}
