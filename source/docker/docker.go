package docker

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"runtime"
	"slices"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	enginecontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/events"
	engineapi "github.com/moby/moby/client"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/vishvananda/netns"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const (
	fieldPodName  = "ERU_POD"
	fieldNodeName = "ERU_NODE_NAME"

	defaultNIC       = "eth0"
	attachBufferSize = 10 << 20
)

var _ source.Source = (*Docker)(nil)

type Docker struct {
	client *engineapi.Client
	config *types.Config

	nodeIP string
	filter engineapi.Filters
}

func New(ctx context.Context, config *types.Config, nodeIP, storeIdentifier string) (*Docker, error) {
	logger := log.WithFunc("docker.New").WithField("nodeIP", nodeIP)
	logger.Info(ctx, "docker source starting")

	client, err := utils.MakeDockerClient(config)
	if err != nil {
		logger.Error(ctx, err, "failed to make docker client")
		return nil, err
	}
	return &Docker{
		client: client,
		config: config,
		nodeIP: nodeIP,
		filter: newFilter(config, storeIdentifier),
	}, nil
}

func (d *Docker) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("docker.List")

	var listed engineapi.ContainerListResult
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		listed, err = d.client.ContainerList(ctx, engineapi.ContainerListOptions{Filters: d.filter, All: true})
	})
	if err != nil {
		logger.Error(ctx, err, "failed to list workloads")
		return nil, err
	}

	workloads := make([]*source.Workload, 0, len(listed.Items))
	for _, c := range listed.Items {
		w, err := d.Get(ctx, c.ID)
		if err != nil {
			logger.WithField("ID", c.ID).Debugf(ctx, "skipping workload: %v", err)
			continue
		}
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (d *Docker) Get(ctx context.Context, ID string) (*source.Workload, error) {
	var inspected engineapi.ContainerInspectResult
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		inspected, err = d.client.ContainerInspect(ctx, ID, engineapi.ContainerInspectOptions{})
	})
	if err != nil {
		return nil, err
	}

	c := inspected.Container
	labels := c.Config.Labels
	if _, ok := labels[cluster.ERUMark]; !ok {
		return nil, common.ErrInvalidContainer
	}
	if d.config.CheckOnlyMine && !utils.UseLabelAsFilter() && !d.checkHostname(c.Config.Env) {
		return nil, common.ErrInvalidContainer
	}

	appname, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}
	env := normalizeEnv(c.Config.Env)
	meta := coreutils.DecodeMetaInLabel(ctx, labels)

	w := &source.Workload{
		ID: c.ID,
		Meta: source.Meta{
			Appname:     appname,
			Entrypoint:  entrypoint,
			Ident:       ident,
			Podname:     env[fieldPodName],
			Nodename:    env[fieldNodeName],
			CoreID:      labels[cluster.LabelCoreID],
			Labels:      labels,
			HealthCheck: meta.HealthCheck,
			Publish:     meta.Publish,
		},
		Running: c.State.Running && c.State.Pid != 0,
	}
	if !w.Running {
		return w, nil
	}

	w.NetnsPID = c.State.Pid
	w.CgroupPath = cgroupPath(ctx, c.State.Pid)
	if c.NetworkSettings != nil {
		w.LocalIP, w.Meta.Networks = d.workloadNetworks(ctx, c)
	}
	return w, nil
}

func (d *Docker) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		f := maps.Clone(d.filter).Add("type", string(events.ContainerEventType))
		stream := d.client.Events(ctx, engineapi.EventsListOptions{Filters: f})
		for {
			select {
			case message := <-stream.Messages:
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

func (d *Docker) Alive(ctx context.Context) bool {
	var err error
	utils.WithTimeout(ctx, d.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = d.client.Ping(ctx, engineapi.PingOptions{})
	})
	if err != nil {
		log.WithFunc("docker.Alive").Error(ctx, err, "connect to docker daemon failed")
		return false
	}
	return true
}

func (d *Docker) Attach(ctx context.Context, ID string) (io.Reader, io.Reader, error) {
	logger := log.WithFunc("docker.Attach").WithField("ID", ID)
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

func (d *Docker) checkHostname(env []string) bool {
	return slices.ContainsFunc(env, func(e string) bool {
		name, value, ok := strings.Cut(e, "=")
		return ok && name == fieldNodeName && value == d.config.HostName
	})
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

func newFilter(config *types.Config, storeIdentifier string) engineapi.Filters {
	labels := map[string]string{cluster.ERUMark: "1"}
	if config.CheckOnlyMine && utils.UseLabelAsFilter() {
		labels[cluster.LabelNodeName] = config.HostName
		if storeIdentifier != "" {
			labels[cluster.LabelCoreID] = storeIdentifier
		}
	}

	f := make(engineapi.Filters)
	for key, value := range labels {
		f.Add("label", fmt.Sprintf("%s=%s", key, value))
	}
	return f
}

func normalizeEnv(env []string) map[string]string {
	em := make(map[string]string, len(env))
	for _, e := range env {
		name, value, _ := strings.Cut(e, "=")
		em[name] = value
	}
	return em
}

func cgroupPath(ctx context.Context, pid int) string {
	path, err := utils.CgroupPath(utils.CgroupRoot, utils.ProcRoot(), pid)
	if err != nil {
		log.WithFunc("docker.cgroupPath").Warnf(ctx, "no cgroup v2 path for pid %d, this node reports no metrics: %v", pid, err)
		return ""
	}
	return path
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
