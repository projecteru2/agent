package engine

import (
	"context"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

func (e *Engine) load() error {
	log.Info("[load] Load containers")
	containers, err := e.listContainers(true, nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		log.Debugf("[load] detect container %s", coreutils.ShortID(container.ID))
		c, err := e.detectContainer(container.ID)
		if err != nil {
			log.Errorf("[load] detect container failed %v", err)
			continue
		}

		if c.Running {
			//TODO 这里应该用文档说明你丫的远端得有东西
			//if _, ok := container.Labels["agent"]; !ok || !e.dockerized {
			e.attach(c)
			//}
		}

		if err := e.store.SetContainerStatus(context.Background(), c, e.node); err != nil {
			log.Errorf("[load] update deploy status failed %v", err)
		}
	}
	return nil
}
