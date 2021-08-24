package engine

import (
	"context"
	"os"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	corestore "github.com/projecteru2/agent/store/core"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	dockerengine "github.com/projecteru2/core/engine/docker"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"

	engineapi "github.com/docker/docker/client"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
)

// Engine is agent
type Engine struct {
	store   store.Store
	config  *types.Config
	docker  *engineapi.Client
	node    *coretypes.Node
	nodeIP  string
	cpuCore float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧
	memory  int64
	cas     utils.GroupCAS

	transfers *utils.HashBackends
	forwards  *utils.HashBackends

	dockerized bool

	// coreIdentifier indicates which eru this agent belongs to
	// it can be used to identify the corresponding core
	// and all containers that belong to this core
	coreIdentifier string
}

// NewEngine make a engine instance
func NewEngine(ctx context.Context, config *types.Config) (*Engine, error) {
	engine := &Engine{}
	docker, err := utils.MakeDockerClient(config)
	if err != nil {
		return nil, err
	}

	store, err := corestore.New(ctx, config)
	if err != nil {
		return nil, err
	}

	// set core identifier
	engine.coreIdentifier = store.GetCoreIdentifier()

	// get self
	node, err := store.GetNode(config.HostName)
	if err != nil {
		return nil, err
	}

	engine.config = config
	engine.store = store
	engine.docker = docker
	engine.node = node
	engine.nodeIP = dockerengine.GetIP(ctx, node.Endpoint)
	if engine.nodeIP == "" {
		engine.nodeIP = common.LocalIP
	}
	log.Infof("[NewEngine] Host IP %s", engine.nodeIP)
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

// Run will start agent
// blocks by ctx.Done()
// either call this in a separated goroutine, or used in main to block main goroutine
func (e *Engine) Run(ctx context.Context) error {
	// load container
	if err := e.load(ctx); err != nil {
		return err
	}
	// start status watcher
	eventChan, errChan := e.initMonitor()
	go e.monitor(eventChan)

	// start health check
	go e.healthCheck(ctx)

	// start node heartbeat
	go e.heartbeat(ctx)

	log.Info("[Engine] Node activated")

	// wait for signal
	select {
	case <-ctx.Done():
		log.Info("[Engine] Agent caught system signal, exiting")
		return nil
	case err := <-errChan:
		if err := e.crash(ctx); err != nil {
			log.Infof("[Engine] Mark node crash failed %v", err)
		}
		return err
	}
}

func (e *Engine) crash(ctx context.Context) error {
	log.Info("[crash] mark all containers unhealthy")
	containers, err := e.listContainers(false, nil)
	if err != nil {
		return err
	}
	for _, c := range containers {
		container, err := e.detectContainer(ctx, c.ID)
		if err != nil {
			return err
		}
		container.Healthy = false

		if err := e.setContainerStatus(container); err != nil {
			return err
		}
		log.Infof("[crash] mark %s unhealthy", coreutils.ShortID(container.ID))
	}
	return e.activated(false)
}
