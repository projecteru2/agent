package engine

import (
	"context"

	enginetypes "github.com/docker/docker/api/types"
	enginefilters "github.com/docker/docker/api/types/filters"
)

func (e *Engine) ListContainers(all bool) ([]enginetypes.Container, error) {
	f := enginefilters.NewArgs()
	f.Add("label", "ERU=1")
	opts := enginetypes.ContainerListOptions{Filters: f, All: all}
	return e.docker.ContainerList(context.Background(), opts)
}
