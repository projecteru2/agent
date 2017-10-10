package engine

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/Sirupsen/logrus"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/store/core"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"
)

type Engine struct {
	store   store.Store
	config  *types.Config
	docker  *engineapi.Client
	node    *coretypes.Node
	cpuCore float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧

	transfers *utils.HashBackends
	forwards  *utils.HashBackends

	dockerized bool
}

func NewEngine(config *types.Config) (*Engine, error) {
	engine := &Engine{}
	docker, err := utils.MakeDockerClient(config)
	if err != nil {
		return nil, err
	}

	store, err := corestore.NewClient(config)
	if err != nil {
		return nil, err
	}

	engine.config = config
	engine.store = store
	engine.docker = docker
	engine.dockerized = os.Getenv(common.DOCKERIZED) != ""
	engine.cpuCore = float64(runtime.NumCPU())
	engine.transfers = utils.NewHashBackends(config.Metrics.Transfers)
	engine.forwards = utils.NewHashBackends(config.Log.Forwards)
	return engine, nil
}

func (e *Engine) Run() error {
	// get self
	node, err := e.store.GetNode(e.config.HostName)
	if err != nil {
		return err
	}

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
	node.Available = true
	if err := e.store.UpdateNode(node); err != nil {
		return err
	}
	log.Info("[Engine] Node activated")

	// wait for signal
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("Agent caught system signal %s, exiting", s)
		return nil
	case err := <-errChan:
		e.store.Crash(node)
		return err
	}
}
