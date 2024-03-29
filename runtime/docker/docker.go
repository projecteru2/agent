package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http/httputil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/vishvananda/netns"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	enginefilters "github.com/docker/docker/api/types/filters"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/log"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// Docker .
type Docker struct {
	client *engineapi.Client
	config *types.Config

	nodeIP    string
	cpuCore   float64 // 因为到时候要乘以 float64 所以就直接转换成 float64 吧
	memory    int64
	cas       *utils.GroupCAS
	transfers *utils.HashBackends
}

const (
	fieldPodName         = "ERU_POD"
	fieldNodeName        = "ERU_NODE_NAME"
	fieldStoreIdentifier = "eru.coreid"
)

// New returns a wrapper of docker client
func New(ctx context.Context, config *types.Config, nodeIP string) (*Docker, error) {
	d := &Docker{
		config:    config,
		cas:       utils.NewGroupCAS(),
		nodeIP:    nodeIP,
		transfers: utils.NewHashBackends(config.Metrics.Transfers),
	}
	logger := log.WithFunc("runtime.docker.New").WithField("nodeIP", d.nodeIP)

	logger.Infof(ctx, "Host IP %s", d.nodeIP)
	var err error
	if d.client, err = utils.MakeDockerClient(config); err != nil {
		logger.Error(ctx, err, "failed to make docker client")
		return nil, err
	}

	if utils.IsDockerized() {
		os.Setenv("HOST_PROC", "/hostProc")
	}

	cpus, err := cpu.Info()
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Host has %d cpus", len(cpus))

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Host has %d memory", memory.Total)

	d.cpuCore = float64(len(cpus))
	d.memory = int64(memory.Total)
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
		log.WithFunc("ListWorkloadIDs").Error(ctx, err, "failed to list workloads")
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
	logger := log.WithFunc("AttachWorkload").WithField("ID", ID)
	resp, err := d.client.ContainerAttach(ctx, ID, enginetypes.ContainerAttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
	})
	if err != nil && err != httputil.ErrPersistEOF { //nolint
		logger.Error(ctx, err, "failed to attach workload")
		return nil, nil, err
	}

	capacity, _ := units.RAMInBytes("10M")
	outr, outw := utils.NewBufPipe(capacity)
	errr, errw := utils.NewBufPipe(capacity)

	_ = utils.Pool.Submit(func() {
		defer func() {
			resp.Close()
			outw.Close()
			errw.Close()
			outr.Close()
			errr.Close()
			logger.Debug(ctx, "buf pipes closed")
		}()

		if _, err = stdcopy.StdCopy(outw, errw, resp.Reader); err != nil {
			logger.Error(ctx, err, "attach get stream failed")
		}
		logger.Info(ctx, "attach workload finished")
	})

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

func getAddrsFromNS(cid string, ifname string) ([]net.Addr, error) {
	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Save the current network namespace
	origns, _ := netns.Get()
	defer origns.Close()
	defer netns.Set(origns) //nolint:errcheck

	containerNS, err := netns.GetFromDocker(cid)
	if err != nil {
		return nil, err
	}
	defer containerNS.Close()

	if err := netns.Set(containerNS); err != nil {
		return nil, err
	}
	eth0, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, err
	}
	addrs, err := eth0.Addrs()
	if err != nil {
		return nil, err
	}
	return addrs, nil
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
		return nil, common.ErrInvaildContainer
	}

	// TODO should be removed in the future
	if d.config.CheckOnlyMine && !utils.UseLabelAsFilter() && !d.checkHostname(c.Config.Env) {
		return nil, common.ErrInvaildContainer
	}

	// 生成基准 meta
	meta := coreutils.DecodeMetaInLabel(ctx, label)

	// 是否符合 eru pattern，如果一个容器又有 ERUMark 又是三段式的 name，那它就是个 ERU 容器
	container, err := generateContainerMeta(ctx, c, meta, label)
	if err != nil {
		return nil, err
	}
	// 计算容器用了多少 CPU
	container = calcuateCPUNum(container, c, d.cpuCore)
	if container.Memory == 0 || container.Memory == math.MaxInt64 {
		container.Memory = d.memory
	}
	// 活着才有发布必要
	if c.NetworkSettings != nil && container.Running { //nolint:nestif
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
			if networks[name] == "" {
				addrs, err := getAddrsFromNS(c.ID, "eth0")
				if err != nil {
					log.Error(ctx, err, "failed to get eth0 addrs")
				}
				if len(addrs) > 0 {
					ip, _, err := net.ParseCIDR(addrs[0].String())
					if err == nil {
						container.LocalIP = ip.String()
						networks[name] = ip.String()
					} else {
						log.Error(ctx, err, "failed to parse cidr %s", addrs[0].String())
					}
				}
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

	_ = utils.Pool.Submit(func() {
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
	})

	return eventChan, errChan
}

// GetStatus checks workload's status first, then returns workload status
func (d *Docker) GetStatus(ctx context.Context, ID string, checkHealth bool) (*types.WorkloadStatus, error) {
	logger := log.WithFunc("GetStatus").WithField("ID", ID)
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to detect workload")
		return nil, err
	}

	bytes, err := json.Marshal(container.Labels)
	if err != nil {
		logger.Error(ctx, err, "failed to marshal labels")
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
			return nil, common.ErrGetLockFailed
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
		log.WithFunc("GetWorkloadName").WithField("ID", ID).Error(ctx, err, "failed to get container by id")
		return "", err
	}

	return containerJSON.Name, nil
}

// LogFieldsExtra .
func (d *Docker) LogFieldsExtra(ctx context.Context, ID string) (map[string]string, error) {
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.WithFunc("LogFieldsExtra").WithField("ID", ID).Error(ctx, err, "failed to detect container")
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

func (d *Docker) getContainerStats(ctx context.Context, ID string) (*enginetypes.StatsJSON, error) {
	logger := log.WithFunc("getContainerStats").WithField("ID", ID)
	rawStat, err := d.client.ContainerStatsOneShot(ctx, ID)
	if err != nil {
		logger.Error(ctx, err, "failed to get container stats")
		return nil, err
	}
	b, err := io.ReadAll(rawStat.Body)
	if err != nil {
		logger.Error(ctx, err, "failed to read container stats")
		return nil, err
	}
	stats := &enginetypes.StatsJSON{}
	return stats, json.Unmarshal(b, stats)
}

func (d *Docker) getBlkioStats(ctx context.Context, ID string) (*enginetypes.BlkioStats, error) {
	fullStat, err := d.getContainerStats(ctx, ID)
	if err != nil {
		return nil, err
	}
	return &fullStat.BlkioStats, nil
}

// IsDaemonRunning returns if the runtime daemon is running.
func (d *Docker) IsDaemonRunning(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = d.client.Ping(ctx)
	})
	if err != nil {
		log.WithFunc("IsDaemonRunning").Error(ctx, err, "connect to docker daemon failed")
		return false
	}
	return true
}

// Name returns the name of runtime
func (d *Docker) Name() string {
	return "docker"
}
