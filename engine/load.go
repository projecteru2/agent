package engine

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
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
			log.Warnf("%s container %s down", c.Name, c.ID[:7])
			c.Alive = false
			if err := e.bind(c); err != nil {
				log.Errorf("bind container info failed %s", err)
			}
			continue
		}

		stop := make(chan int)
		e.attach(c, stop)
		go e.stat(c, stop)
	}
	return nil
}

func (e *Engine) bind(container *types.Container) error {
	c, err := e.docker.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return err
	}
	container.Pid = c.State.Pid
	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return err
	}
	container.Name = name
	container.EntryPoint = entrypoint
	container.Ident = ident
	return e.store.UpdateContainer(container)
}

func getStatus(s string) string {
	switch {
	case strings.HasPrefix(s, "Up"):
		return common.STATUS_START
	default:
		return common.STATUS_DIE
	}
}
