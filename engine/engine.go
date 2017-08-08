package engine

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/Sirupsen/logrus"
	engineapi "github.com/docker/docker/client"
	"gitlab.ricebook.net/platform/agent/store"
	"gitlab.ricebook.net/platform/agent/store/etcd"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
)

type Engine struct {
	store   store.Store
	config  types.Config
	docker  *engineapi.Client
	cpuCore float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧

	transfers *utils.HashBackends
	forwards  *utils.HashBackends
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
	engine.cpuCore = float64(runtime.NumCPU())
	engine.transfers = utils.NewHashBackends(config.Metrics.Transfers)
	engine.forwards = utils.NewHashBackends(config.Log.Forwards)
	return engine, nil
}

func (e *Engine) Run() error {
	// load container
	if err := e.load(); err != nil {
		return err
	}
	// start status watcher
	eventChan, errChan := e.initMonitor()
	go e.monitor(eventChan)

	// start health check
	go e.healthCheck()

	// tell core this node is ready
	if err := e.store.RegisterNode(&types.Node{Alive: true}); err != nil {
		return err
	}
	log.Info("Node activated")

	// wait for signal
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("Agent caught system signal %s, exiting", s)
		return nil
	case err := <-errChan:
		e.store.Crash()
		return err
	}
}
