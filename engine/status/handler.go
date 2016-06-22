package status

import (
	log "github.com/Sirupsen/logrus"
	eventtypes "github.com/docker/engine-api/types/events"
)

func HandleContainerStart(event eventtypes.Message) {
	log.Info(event)
}

func HandleContainerDie(event eventtypes.Message) {
	log.Info(event)
}
