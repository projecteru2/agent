package engine

import (
	log "github.com/Sirupsen/logrus"
	types "github.com/docker/engine-api/types"
	eventtypes "github.com/docker/engine-api/types/events"
	filtertypes "github.com/docker/engine-api/types/filters"
	"golang.org/x/net/context"
	"os"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/engine/status"
)

var eventHandler = status.NewEventHandler()

func (e *Engine) monitor() {
	eventHandler.Handle(common.STATUS_START, e.handleContainerStart)
	eventHandler.Handle(common.STATUS_DIE, e.handleContainerDie)
	eventHandler.Handle(common.STATUS_DESTROY, e.handleContainerDestroy)

	var eventChan = make(chan eventtypes.Message)
	go eventHandler.Watch(eventChan)
	e.monitorContainerEvents(eventChan)
	close(eventChan)
}

func (e *Engine) monitorContainerEvents(c chan eventtypes.Message) {
	ctx := context.Background()
	f := filtertypes.NewArgs()
	f.Add("type", "container")
	options := types.EventsOptions{Filters: f}
	resBody, err := e.docker.Events(ctx, options)
	// Whether we successfully subscribed to events or not, we can now
	// unblock the main goroutine.
	if err != nil {
		e.errChan <- err
		return
	}
	log.Info("Status watch start")
	defer resBody.Close()

	if err := status.DecodeEvents(resBody, c); err != nil {
		e.errChan <- err
	}
}

func (e *Engine) handleContainerStart(event eventtypes.Message) {
	log.Debugf("container %s start", event.ID[:7])
	if _, ok := event.Actor.Attributes["ERU"]; !ok {
		return
	}
	//清理掉 ERU 标志
	delete(event.Actor.Attributes, "ERU")

	//看是否有元数据，有则是 crash 后重启
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		// 看看是不是 etcd 的 KeyNotFound error, 我们只需要对这种做进一步处理
		if etcdErr, ok := err.(client.Error); !ok || etcdErr.Code != client.ErrorCodeKeyNotFound {
			log.Error(err)
			return
		}

		// 找不到说明需要重新从 label 生成数据
		log.Debug(event.Actor.Attributes)
		container, err = status.GenerateContainerMeta(event.ID, event.Actor.Attributes)
		if err != nil {
			log.Error(err)
			return
		}
	}

	c, err := e.docker.ContainerInspect(context.Background(), event.ID)
	if err != nil {
		log.Error(err)
		return
	}

	container.Pid = c.State.Pid
	container.Alive = true
	container.Healthy = e.judgeContainerHealth(c)
	log.Debug(container)
	if err := e.bind(container); err != nil {
		log.Error(err)
		return
	}

	stop := make(chan int)
	e.attach(container, stop)
	go e.stat(container, stop)
}

func (e *Engine) handleContainerDie(event eventtypes.Message) {
	log.Debugf("container %s die", event.ID[:7])
	container, err := e.store.GetContainer(event.ID)
	if err != nil {
		log.Error(err)
		return
	}
	container.Alive = false
	container.Healthy = false
	if err := e.store.UpdateContainer(container); err != nil {
		log.Error(err)
	}
}

func (e *Engine) handleContainerDestroy(event eventtypes.Message) {
	log.Debugf("container %s destroy", event.ID[:7])
	_, err := e.store.GetContainer(event.ID)
	if err != nil {
		hostname := os.Getenv("HOSTNAME")
		log.Errorf("while geting container from etcd, %s occured err: %v", hostname, err)
	}
	if err := e.store.RemoveContainer(event.ID); err != nil {
		hostname := os.Getenv("HOSTNAME")
		log.Errorf("while removing container, %s occured err: %v", hostname, err)
	}
	log.Debugf("container %s data removed", event.ID[:7])
}
