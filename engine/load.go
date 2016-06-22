package engine

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/agent/common"
	"golang.org/x/net/context"
)

func (e *Engine) load() error {
	log.Info("Load containers")
	ctx := context.Background()
	options := enginetypes.ContainerListOptions{All: true}

	containers, err := e.docker.ContainerList(ctx, options)
	if err != nil {
		return err
	}

	for _, container := range containers {
		c, err := e.store.GetContainer(container.ID)
		if err != nil {
			log.Debugf("Load container stats failed %s", err)
			continue
		}
		if c == nil {
			continue
		}
		status := getStatus(container.Status)
		if status != common.STATUS_START {
			log.Infof("container %s down", c.ID[:7])
			c.Alive = false
			e.store.UpdateContainer(c)
			continue
		}

		e.attach(c)
		//go c.Metrics()
	}
	return nil
}

func getStatus(s string) string {
	switch {
	case strings.HasPrefix(s, "Up"):
		return common.STATUS_START
	default:
		return common.STATUS_DIE
	}
}
