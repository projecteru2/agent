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

	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/vishvananda/netns"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	enginefilters "github.com/docker/docker/api/types/filters"
	engineapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	fieldPodName         = "ERU_POD"
	fieldNodeName        = "ERU_NODE_NAME"
	fieldStoreIdentifier = "eru.coreid"

	defaultNIC = "eth0"
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
	logger := log.WithFunc("runtime.docker.New").WithField("nodeIP", d.nodeIP)

	logger.Infof(ctx, "Host IP %s", d.nodeIP)
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
	d.memory = int64(min(memory.Total, math.MaxInt64))
	return d, nil
}

func (d *Docker) ListWorkloadIDs(ctx context.Context, filters map[string]string) ([]string, error) {
	f := d.getFilterArgs(filters)
	opts := enginecontainer.ListOptions{Filters: f, All: true}

	var containers []enginecontainer.Summary
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

func (d *Docker) AttachWorkload(ctx context.Context, ID string) (io.Reader, io.Reader, error) {
	logger := log.WithFunc("AttachWorkload").WithField("ID", ID)
	resp, err := d.client.ContainerAttach(ctx, ID, enginecontainer.AttachOptions{
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

		f := d.getFilterArgs(filters)
		f.Add("type", string(events.ContainerEventType))
		options := events.ListOptions{Filters: f}
		m, e := d.client.Events(ctx, options)
		for {
			select {
			case message := <-m:
				eventChan <- &types.WorkloadEventMessage{
					ID:       message.Actor.ID,
					Type:     string(message.Type),
					Action:   string(message.Action),
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
	var containerJSON enginecontainer.InspectResponse
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

func (d *Docker) Name() string {
	return "docker"
}

func (d *Docker) getFilterArgs(filters map[string]string) enginefilters.Args {
	f := enginefilters.NewArgs()

	for key, value := range filters {
		f.Add("label", fmt.Sprintf("%s=%s", key, value))
	}

	return f
}

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

func (d *Docker) detectWorkload(ctx context.Context, ID string) (*Container, error) {
	var c enginecontainer.InspectResponse
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

	if d.config.CheckOnlyMine && !utils.UseLabelAsFilter() && !d.checkHostname(c.Config.Env) {
		return nil, common.ErrInvaildContainer
	}

	meta := coreutils.DecodeMetaInLabel(ctx, label)

	container, err := generateContainerMeta(ctx, c, meta, label)
	if err != nil {
		return nil, err
	}
	container = calcuateCPUNum(container, c, d.cpuCore)
	if container.Memory == 0 || container.Memory == math.MaxInt64 {
		container.Memory = d.memory
	}
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
				if ip := addrFromNS(ctx, c.ID, defaultNIC); ip != "" {
					container.LocalIP = ip
					networks[name] = ip
				}
			}
			break
		}
		container.Networks = networks
	}

	return container, nil
}

func (d *Docker) getContainerStats(ctx context.Context, ID string) (*enginecontainer.StatsResponse, error) {
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
	logger := log.WithFunc("addrFromNS").WithField("ID", cid)
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
		logger.Error(ctx, err, "failed to find %s", ifname)
		return ""
	}
	addrs, err := nic.Addrs()
	if err != nil {
		logger.Error(ctx, err, "failed to get %s addrs", ifname)
		return ""
	}
	if len(addrs) == 0 {
		return ""
	}
	ip, _, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		logger.Error(ctx, err, "failed to parse cidr %s", addrs[0].String())
		return ""
	}
	return ip.String()
}
