package engine

import (
	"context"

	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

func (e *Engine) load(ctx context.Context) error {
	log.Info("[load] Load containers")
	containers, err := e.listContainers(true, nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		log.Debugf("[load] detect container %s", coreutils.ShortID(container.ID))
		c, err := e.detectContainer(ctx, container.ID)
		if err != nil {
			log.Errorf("[load] detect container failed %v", err)
			continue
		}

		if c.Running {
			e.attach(c)
		}

		if err := e.setContainerStatus(c); err != nil {
			log.Errorf("[load] update deploy status failed %v", err)
		}
	}
	return nil
}
