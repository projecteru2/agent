package engine

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/context"

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
	// check docker alive
	_, err := e.docker.Info(context.Background())
	if err != nil {
		log.Panicf("Docker down", err)
	}

	// load container
	if err := e.load(); err != nil {
		log.Panicf("Eru Agent load failed %s", err)
	}
	// start status watcher
	go e.monitor()

	// tell core this node is ready
	if err := e.store.RegisterNode(&types.Node{Alive: true}); err != nil {
		log.Panic(err)
	}
	log.Info("Node activated")

	// wait for signal
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("Eru Agent Catch %s", s)
		return
	case err := <-e.errChan:
		e.store.Crash()
		log.Panicf("Eru Agent Error %s", err)
	}
}
