package engine

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	engineapi "github.com/docker/engine-api/client"
	"gitlab.ricebook.net/platform/agent/store"
	"gitlab.ricebook.net/platform/agent/store/etcd"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
)

type Engine struct {
	store   store.Store
	config  types.Config
	docker  *engineapi.Client
	errChan chan error
}

func NewEngine(config types.Config) (*Engine, error) {
	engine := &Engine{}
	store, err := etcdstore.NewClient(config)
	if err != nil {
		return nil, err
	}
	docker, err := utils.MakeDockerClient(config)
	if err != nil {
		return nil, err
	}
	engine.config = config
	engine.store = store
	engine.docker = docker
	engine.errChan = make(chan error)
	return engine, nil
}

func (e *Engine) Run() {
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("Eru Agent Catch %s", s)
		return
	case e := <-e.errChan:
		e.store.Crash()
		log.Panicf("Eru Agent Error %s", e)
	}
}
