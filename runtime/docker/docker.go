package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	enginecontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	engineapi "github.com/moby/moby/client"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/vishvananda/netns"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const (
	fieldPodName         = "ERU_POD"
	fieldNodeName        = "ERU_NODE_NAME"
	fieldStoreIdentifier = "eru.coreid"

	defaultNIC       = "eth0"
	attachBufferSize = 10 << 20
)

type Docker struct {
	client *engineapi.Client
	config *types.Config

	nodeIP    string
	cpuCore   float64
	memory    int64
	cas       *utils.GroupCAS
	transfers *utils.HashBackends
}

func New(ctx context.Context, config *types.Config, nodeIP string) (*Docker, error) {
	d := &Docker{
		config:    config,
		cas:       utils.NewGroupCAS(),
		nodeIP:    nodeIP,
		transfers: utils.NewHashBackends(config.Metrics.Transfers),
	}
	logger := log.WithFunc("docker.New").WithField("nodeIP", d.nodeIP)

	logger.Info(ctx, "docker runtime starting")
	var err error
	if d.client, err = utils.MakeDockerClient(config); err != nil {
		logger.Error(ctx, err, "failed to make docker client")
		return nil, err
	}

	if utils.IsDockerized() {
		if err = os.Setenv("HOST_PROC", "/hostProc"); err != nil {
			return nil, err
		}
	}

	cpus := runtime.NumCPU()
	logger.Infof(ctx, "host has %d cpus", cpus)

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	logger.Infof(ctx, "host has %d bytes of memory", memory.Total)

	d.cpuCore = float64(cpus)
	d.memory = int64(min(memory.Total, math.MaxInt64))
	return d, nil
}

func (d *Docker) ListWorkloadIDs(ctx context.Context, filters map[string]string) ([]string, error) {
	opts := engineapi.ContainerListOptions{Filters: d.getFilterArgs(filters), All: true}

	var listed engineapi.ContainerListResult
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		listed, err = d.client.ContainerList(ctx, opts)
	})
	if err != nil {
		log.WithFunc("docker.ListWorkloadIDs").Error(ctx, err, "failed to list workloads")
		return nil, err
	}

	workloads := make([]string, 0, len(listed.Items))
	for _, c := range listed.Items {
		workloads = append(workloads, c.ID)
	}
	return workloads, nil
}

func (d *Docker) AttachWorkload(ctx context.Context, ID string) (io.Reader, io.Reader, error) {
	logger := log.WithFunc("docker.AttachWorkload").WithField("ID", ID)
	resp, err := d.client.ContainerAttach(ctx, ID, engineapi.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		logger.Error(ctx, err, "failed to attach workload")
		return nil, nil, err
	}

	outr, outw := utils.NewBufPipe(attachBufferSize)
	errr, errw := utils.NewBufPipe(attachBufferSize)

	go func() {
		defer func() {
			resp.Close()
			_ = outw.Close()
			_ = errw.Close()
			_ = outr.Close()
			_ = errr.Close()
			logger.Debug(ctx, "buf pipes closed")
		}()

		if _, err = stdcopy.StdCopy(outw, errw, resp.Reader); err != nil {
			logger.Error(ctx, err, "attach get stream failed")
		}
		logger.Info(ctx, "attach workload finished")
	}()

	return outr, errr, nil
}

func (d *Docker) Events(ctx context.Context, filters map[string]string) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		f := d.getFilterArgs(filters).Add("type", string(events.ContainerEventType))
		stream := d.client.Events(ctx, engineapi.EventsListOptions{Filters: f})
		for {
			select {
			case message := <-stream.Messages:
				if message.Action == events.ActionUpdate {
					d.refreshMetricsContainer(ctx, message.Actor.ID)
				}
				eventChan <- &types.WorkloadEventMessage{
					ID:       message.Actor.ID,
					Type:     string(message.Type),
					Action:   string(message.Action),
					TimeNano: message.TimeNano,
				}
			case err := <-stream.Err:
				errChan <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, errChan
}

func (d *Docker) GetStatus(ctx context.Context, ID string, checkHealth bool) (*types.WorkloadStatus, error) {
	logger := log.WithFunc("docker.GetStatus").WithField("ID", ID)
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

func (d *Docker) GetWorkloadName(ctx context.Context, ID string) (string, error) {
	var inspected engineapi.ContainerInspectResult
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		inspected, err = d.client.ContainerInspect(ctx, ID, engineapi.ContainerInspectOptions{})
	})
	if err != nil {
		log.WithFunc("docker.GetWorkloadName").WithField("ID", ID).Error(ctx, err, "failed to get container by id")
		return "", err
	}

	return inspected.Container.Name, nil
}

func (d *Docker) LogFieldsExtra(ctx context.Context, ID string) (map[string]string, error) {
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.WithFunc("docker.LogFieldsExtra").WithField("ID", ID).Error(ctx, err, "failed to detect container")
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

func (d *Docker) IsDaemonRunning(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = d.client.Ping(ctx, engineapi.PingOptions{})
	})
	if err != nil {
		log.WithFunc("docker.IsDaemonRunning").Error(ctx, err, "connect to docker daemon failed")
		return false
	}
	return true
}

func (d *Docker) getFilterArgs(filters map[string]string) engineapi.Filters {
	f := make(engineapi.Filters)

	for key, value := range filters {
		f.Add("label", fmt.Sprintf("%s=%s", key, value))
	}

	return f
}

func (d *Docker) checkHostname(env []string) bool {
	return slices.ContainsFunc(env, func(e string) bool {
		name, value, ok := strings.Cut(e, "=")
		return ok && name == fieldNodeName && value == d.config.HostName
	})
}

func (d *Docker) detectWorkload(ctx context.Context, ID string) (*Container, error) {
	var inspected engineapi.ContainerInspectResult
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		inspected, err = d.client.ContainerInspect(ctx, ID, engineapi.ContainerInspectOptions{})
	})
	if err != nil {
		return nil, err
	}
	c := inspected.Container
	label := c.Config.Labels

	if _, ok := label[cluster.ERUMark]; !ok {
		return nil, common.ErrInvalidContainer
	}

	if d.config.CheckOnlyMine && !utils.UseLabelAsFilter() && !d.checkHostname(c.Config.Env) {
		return nil, common.ErrInvalidContainer
	}

	meta := coreutils.DecodeMetaInLabel(ctx, label)

	container, err := generateContainerMeta(ctx, c, meta, label)
	if err != nil {
		return nil, err
	}
	container = calculateCPUNum(container, c, d.cpuCore)
	if container.Memory == 0 || container.Memory == math.MaxInt64 {
		container.Memory = d.memory
	}
	if c.NetworkSettings != nil && container.Running {
		container.LocalIP, container.Networks = d.workloadNetworks(ctx, c)
	}

	return container, nil
}

func (d *Docker) refreshMetricsContainer(ctx context.Context, ID string) {
	container, err := d.detectWorkload(ctx, ID)
	if err != nil {
		log.WithFunc("docker.refreshMetricsContainer").WithField("ID", ID).Error(ctx, err, "failed to refresh container meta")
		return
	}
	setMetricsContainer(ID, container)
}

func (d *Docker) workloadNetworks(ctx context.Context, c enginecontainer.InspectResponse) (string, map[string]string) {
	networks := map[string]string{}
	names := slices.Sorted(maps.Keys(c.NetworkSettings.Networks))
	if len(names) == 0 {
		return "", networks
	}

	name := names[0]
	addr := ""
	if ip := c.NetworkSettings.Networks[name].IPAddress; ip.IsValid() {
		addr = ip.String()
	}
	localIP := addr
	if enginecontainer.NetworkMode(name).IsHost() {
		localIP, addr = common.LocalIP, d.nodeIP
	}
	if addr == "" {
		if ip := addrFromNS(ctx, c.ID, defaultNIC); ip != "" {
			localIP, addr = ip, ip
		}
	}
	networks[name] = addr

	return localIP, networks
}

func (d *Docker) getContainerStats(ctx context.Context, ID string) (*enginecontainer.StatsResponse, error) {
	logger := log.WithFunc("docker.getContainerStats").WithField("ID", ID)
	rawStat, err := d.client.ContainerStats(ctx, ID, engineapi.ContainerStatsOptions{})
	if err != nil {
		logger.Error(ctx, err, "failed to get container stats")
		return nil, err
	}
	defer func() { _ = rawStat.Body.Close() }()
	b, err := io.ReadAll(rawStat.Body)
	if err != nil {
		logger.Error(ctx, err, "failed to read container stats")
		return nil, err
	}
	stats := &enginecontainer.StatsResponse{}
	return stats, json.Unmarshal(b, stats)
}

func (d *Docker) getBlkioStats(ctx context.Context, ID string) (*enginecontainer.BlkioStats, error) {
	fullStat, err := d.getContainerStats(ctx, ID)
	if err != nil {
		return nil, err
	}
	return &fullStat.BlkioStats, nil
}

func addrFromNS(ctx context.Context, cid, ifname string) string {
	logger := log.WithFunc("docker.addrFromNS").WithField("ID", cid)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, _ := netns.Get()
	defer func() {
		_ = netns.Set(origns)
		_ = origns.Close()
	}()

	containerNS, err := netns.GetFromDocker(cid)
	if err != nil {
		logger.Error(ctx, err, "failed to get the workload netns")
		return ""
	}
	defer func() { _ = containerNS.Close() }()

	if err = netns.Set(containerNS); err != nil {
		logger.Error(ctx, err, "failed to enter the workload netns")
		return ""
	}
	nic, err := net.InterfaceByName(ifname)
	if err != nil {
		logger.Errorf(ctx, err, "failed to find %s", ifname)
		return ""
	}
	addrs, err := nic.Addrs()
	if err != nil {
		logger.Errorf(ctx, err, "failed to get %s addrs", ifname)
		return ""
	}
	if len(addrs) == 0 {
		return ""
	}
	ip, _, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		logger.Errorf(ctx, err, "failed to parse cidr %s", addrs[0].String())
		return ""
	}
	return ip.String()
}
