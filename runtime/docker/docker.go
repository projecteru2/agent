package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	enginefilters "github.com/docker/docker/api/types/filters"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
)

// Docker .
type Docker struct {
	client *engineapi.Client
	config *types.Config

	nodeIP    string
	cpuCore   float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧
	memory    int64
	cas       utils.GroupCAS
	transfers *utils.HashBackends
}

const (
	fieldPodName         = "ERU_POD"
	fieldNodeName        = "ERU_NODE_NAME"
	fieldStoreIdentifier = "eru.coreid"
)

// New returns a wrapper of docker client
func New(config *types.Config, nodeIP string) (*Docker, error) {
	d := &Docker{}
	d.config = config

	var err error
	d.client, err = utils.MakeDockerClient(config)
	if err != nil {
		log.Errorf("[NewDocker] failed to make docker client, err: %v", err)
		return nil, err
	}

	d.nodeIP = nodeIP
	log.Infof("[NewDocker] Host IP %s", d.nodeIP)

	if utils.IsDockerized() {
		os.Setenv("HOST_PROC", "/hostProc")
	}

	cpus, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	log.Infof("[NewDocker] Host has %d cpus", len(cpus))

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	log.Infof("[NewDocker] Host has %d memory", memory.Total)

	d.cpuCore = float64(len(cpus))
	d.memory = int64(memory.Total)

	d.transfers = utils.NewHashBackends(config.Metrics.Transfers)

	return d, nil
}

func (d *Docker) getFilterArgs(filters map[string]string) enginefilters.Args {
	f := enginefilters.NewArgs()

	for key, value := range filters {
		f.Add("label", fmt.Sprintf("%s=%s", key, value))
	}

	return f
}

// ListWorkloadIDs lists workload IDs filtered by given condition
func (d *Docker) ListWorkloadIDs(ctx context.Context, filters map[string]string) ([]string, error) {
	f := d.getFilterArgs(filters)
	opts := enginetypes.ContainerListOptions{Filters: f, All: true}

	var containers []enginetypes.Container
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		containers, err = d.client.ContainerList(ctx, opts)
	})
	if err != nil {
		log.Errorf("[ListWorkloadIDs] failed to list workloads, err: %v", err)
		return nil, err
	}

	workloads := make([]string, 0, len(containers))
	for _, c := range containers {
		workloads = append(workloads, c.ID)
	}
	return workloads, nil
}

// AttachWorkload .
func (d *Docker) AttachWorkload(ctx context.Context, ID string) (io.Reader, io.Reader, error) {
	resp, err := d.client.ContainerAttach(ctx, ID, enginetypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
	})
	if err != nil && err != httputil.ErrPersistEOF { // nolint
		log.Errorf("[AttachWorkload] failed to attach workload %v, err: %v", ID, err)
		return nil, nil, err
	}

	cap, _ := units.RAMInBytes("10M")
	outr, outw := utils.NewBufPipe(cap)
	errr, errw := utils.NewBufPipe(cap)

	go func() {
		defer func() {
			resp.Close()
			outw.Close()
			errw.Close()
			outr.Close()
			errr.Close()
			log.Debugf("[attach] %v buf pipes closed", ID)
		}()

		_, err = stdcopy.StdCopy(outw, errw, resp.Reader)
		if err != nil {
			log.Errorf("[attach] attach get stream failed %s", err)
		}
		log.Infof("[attach] attach workload %s finished", ID)
	}()

	return outr, errr, nil
}

// checkHostname check if ERU_NODE_NAME env in container is the hostname of this agent
// TODO should be removed in the future, should always use label to filter
func (d *Docker) checkHostname(env []string) bool {
	for _, e := range env {
		ps := strings.SplitN(e, "=", 2)
		if len(ps) != 2 {
			continue
		}
		if ps[0] == "ERU_NODE_NAME" && ps[1] == d.config.HostName {
			return true
		}
	}
	return false
}

// detectWorkload detect a container by ID
func (d *Docker) detectWorkload(ctx context.Context, ID string) (*Container, error) {
	// 标准化为 inspect 的数据
	var c enginetypes.ContainerJSON
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		c, err = d.client.ContainerInspect(ctx, ID)
	})
	if err != nil {
		return nil, err
	}
	label := c.Config.Labels

	if _, ok := label[cluster.ERUMark]; !ok {
		return nil, fmt.Errorf("not a eru container %s", ID)
	}

	// TODO should be removed in the future
	if d.config.CheckOnlyMine && !utils.UseLabelAsFilter() && !d.checkHostname(c.Config.Env) {
		return nil, fmt.Errorf("should ignore this container")
	}

	// 生成基准 meta
	meta := coreutils.DecodeMetaInLabel(ctx, label)

	// 是否符合 eru pattern，如果一个容器又有 ERUMark 又是三段式的 name，那它就是个 ERU 容器
	container, err := generateContainerMeta(c, meta, label)
	if err != nil {
		return nil, err
	}
	// 计算容器用了多少 CPU
	container = calcuateCPUNum(container, c, d.cpuCore)
	if container.Memory == 0 || container.Memory == math.MaxInt64 {
		container.Memory = d.memory
	}
	// 活着才有发布必要
	if c.NetworkSettings != nil && container.Running {
		networks := map[string]string{}
		for name, endpoint := range c.NetworkSettings.Networks {
			networkmode := enginecontainer.NetworkMode(name)
			if networkmode.IsHost() {
				container.LocalIP = common.LocalIP
				networks[name] = d.nodeIP
			} else {
				container.LocalIP = endpoint.IPAddress
				networks[name] = endpoint.IPAddress
			}
			break
		}
		container.Networks = networks
	}

	return container, nil
}

// Events returns the events of workloads' changes
func (d *Docker) Events(ctx context.Context, filters map[string]string) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		f := d.getFilterArgs(filters)
		f.Add("type", events.ContainerEventType)
		options := enginetypes.EventsOptions{Filters: f}
		m, e := d.client.Events(ctx, options)
		for {
			select {
			case message := <-m:
				eventChan <- &types.WorkloadEventMessage{
					ID:       message.ID,
					Type:     message.Type,
					Action:   message.Action,
					TimeNano: message.TimeNano,
				}
			case err := <-e:
				errChan <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, errChan
}

// GetStatus checks workload's status first, then returns workload status
func (d *Docker) GetStatus(ctx context.Context, ID string, checkHealth bool) (*types.WorkloadStatus, error) {
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.Errorf("[GetStatus] failed to detect workload %v, err: %v", ID, err)
		return nil, err
	}

	bytes, err := json.Marshal(container.Labels)
	if err != nil {
		log.Errorf("[GetStatus] failed to marshal labels, err: %v", err)
		return nil, err
	}

	status := &types.WorkloadStatus{
		ID:         container.ID,
		Running:    container.Running,
		Networks:   container.Networks,
		Extension:  bytes,
		Appname:    container.Name,
		Nodename:   d.config.HostName,
		Entrypoint: container.Entrypoint,
		Healthy:    container.Running && container.HealthCheck == nil,
	}

	// only check the running containers
	if checkHealth && container.Running {
		free, acquired := d.cas.Acquire(container.ID)
		if !acquired {
			return nil, fmt.Errorf("[GetStatus] failed to get the lock")
		}
		defer free()
		status.Healthy = container.CheckHealth(ctx, time.Duration(d.config.HealthCheck.Timeout)*time.Second)
	}

	return status, nil
}

// GetWorkloadName returns the name of workload
func (d *Docker) GetWorkloadName(ctx context.Context, ID string) (string, error) {
	var containerJSON enginetypes.ContainerJSON
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		containerJSON, err = d.client.ContainerInspect(ctx, ID)
	})
	if err != nil {
		log.Errorf("[GetWorkloadName] failed to get container by id %v, err: %v", ID, err)
		return "", err
	}

	return containerJSON.Name, nil
}

// LogFieldsExtra .
func (d *Docker) LogFieldsExtra(ctx context.Context, ID string) (map[string]string, error) {
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.Errorf("[LogFieldsExtra] failed to detect container %v, err: %v", ID, err)
		return nil, err
	}

	extra := map[string]string{
		"podname":  container.Env[fieldPodName],
		"nodename": container.Env[fieldNodeName],
		"coreid":   container.Labels[fieldStoreIdentifier],
	}
	for name, addr := range container.Networks {
		extra[fmt.Sprintf("networks_%s", name)] = addr
	}
	return extra, nil
}

// IsDaemonRunning returns if the runtime daemon is running.
func (d *Docker) IsDaemonRunning(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = d.client.Info(ctx)
	})
	if err != nil {
		log.Debugf("[IsDaemonRunning] connect to docker daemon failed, err: %v", err)
		return false
	}
	return true
}

// Name returns the name of runtime
func (d *Docker) Name() string {
	return "docker"
}
