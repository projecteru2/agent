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

	transfers *utils.HashBackends
	forwards  *utils.HashBackends
	physical  *utils.HashBackends
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
	engine.transfers = utils.NewHashBackends(config.Metrics.Transfers)
	engine.forwards = utils.NewHashBackends(config.Log.Forwards)
	engine.physical = utils.NewHashBackends(config.NIC.Physical)
	return engine, nil
}

func (e *Engine) Run() error {
	// check docker alive
	_, err := e.docker.Info(context.Background())
	if err != nil {
		log.Errorf("Docker down %s", err)
		return err
	}

	// load container
	if err := e.load(); err != nil {
		log.Errorf("Eru Agent load failed %s", err)
		return err
	}
	// start status watcher
	go e.monitor()

	// start health check
	go e.healthCheck()

	// tell core this node is ready
	if err := e.store.RegisterNode(&types.Node{Alive: true}); err != nil {
		log.Errorf("register node failed %s", err)
		return err
	}
	log.Info("Node activated")

	// wait for signal
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("Eru Agent Catch %s", s)
		return nil
	case err := <-e.errChan:
		e.store.Crash()
		log.Errorf("Eru Agent Error %s", err)
		return err
	}
}
