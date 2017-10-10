package engine

import (
	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/agent/common"
)

func (e *Engine) load() error {
	log.Info("[load] Load containers")
	containers, err := e.listContainers(true)
	if err != nil {
		return err
	}

	for _, container := range containers {
		log.Debugf("[load] detect container %s", container.ID[:common.SHORTID])
		c, err := e.detectContainer(container.ID, container.Labels)
		if err != nil {
			log.Errorf("[load] detect container failed %v", err)
			continue
		}

		// 非 eru-agent in docker 就转发日志，防止日志循环输出
		if _, ok := container.Labels["agent"]; !ok || !e.dockerized {
			e.attach(c)
		}

		if err := e.store.DeployContainer(c, e.node); err != nil {
			log.Errorf("[load] update deploy status failed %v", err)
		}
	}
	return nil
}
