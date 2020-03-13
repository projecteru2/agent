package engine

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	corestore "github.com/projecteru2/agent/store/core"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
)

//Engine is agent
type Engine struct {
	store   store.Store
	config  *types.Config
	docker  *engineapi.Client
	node    *coretypes.Node
	cpuCore float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧
	memory  int64

	transfers *utils.HashBackends
	forwards  *utils.HashBackends

	dockerized bool
}

//NewEngine make a engine instance
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

	// get self
	node, err := store.GetNode(config.HostName)
	if err != nil {
		return nil, err
	}

	engine.config = config
	engine.store = store
	engine.docker = docker
	engine.node = node
	engine.dockerized = os.Getenv(common.DOCKERIZED) != ""
	if engine.dockerized {
		os.Setenv("HOST_PROC", "/hostProc")
	}
	cpus, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	log.Infof("[NewEngine] Host has %d cpus", len(cpus))
	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	log.Infof("[NewEngine] Host has %d memory", memory.Total)
	engine.cpuCore = float64(len(cpus))
	engine.memory = int64(memory.Total)
	engine.transfers = utils.NewHashBackends(config.Metrics.Transfers)
	engine.forwards = utils.NewHashBackends(config.Log.Forwards)
	return engine, nil
}

//Run will start agent
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
	if err := e.activated(true); err != nil {
		return err
	}
	log.Info("[Engine] Node activated")

	// wait for signal
	var c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	select {
	case s := <-c:
		log.Infof("[Engine] Agent caught system signal %s, exiting", s)
		return nil
	case err := <-errChan:
		e.crash()
		return err
	}
}

func (e *Engine) crash() error {
	log.Info("[crash] mark all containers unhealthy")
	containers, err := e.listContainers(false, nil)
	if err != nil {
		return err
	}
	for _, c := range containers {
		container, err := e.detectContainer(c.ID)
		if err != nil {
			return err
		}
		container.Healthy = false
		if err := e.store.SetContainerStatus(context.Background(), container, e.node); err != nil {
			return err
		}
		log.Infof("[crash] mark %s unhealthy", coreutils.ShortID(container.ID))
	}
	return e.activated(false)
}
