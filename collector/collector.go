package collector

import (
	"cmp"
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const sysNetRoot = "/sys/class/net"

type device struct {
	major uint64
	minor uint64
}

type sample struct {
	cpu  cpuStat
	mem  memStat
	io   []ioStat
	net  []netStat
	host hostCPU
}

// Collector samples the cgroup and netns counters of the workloads a source yields.
type Collector struct {
	config    *types.Config
	transfers *utils.HashBackends

	hostname string
	cpuCores float64
	memTotal uint64
	procRoot string

	devicesMutex sync.Mutex
	devices      map[device]string
}

func New(ctx context.Context, config *types.Config) *Collector {
	c := &Collector{
		config:    config,
		transfers: utils.NewHashBackends(config.Metrics.Transfers),
		hostname:  strings.ReplaceAll(config.HostName, ".", "-"),
		cpuCores:  float64(runtime.NumCPU()),
		procRoot:  utils.ProcRoot(),
		devices:   map[device]string{},
	}

	total, err := hostMemTotal(c.procRoot)
	if err != nil {
		// without a node total the memory percentages of an unlimited workload stay unreported
		log.WithFunc("collector.New").Warnf(ctx, "failed to read the node memory total: %v", err)
	}
	c.memTotal = total
	return c
}

// Collect samples one workload every metrics step until ctx is canceled or the cgroup goes away.
func (c *Collector) Collect(ctx context.Context, w *source.Workload) {
	logger := log.WithFunc("collector.Collect").WithField("ID", w.ID)
	if w.CgroupPath == "" {
		logger.Debug(ctx, "workload has no cgroup, not sampling it")
		return
	}

	prev, err := c.sample(w)
	if err != nil {
		logger.Error(ctx, err, "failed to read the first sample")
		return
	}

	addr := ""
	if c.transfers.Len() > 0 {
		addr = c.transfers.Get(w.ID, 0)
	}
	client := NewMetricsClient(addr, c.hostname, w)
	defer removeMetricsClient(w.ID)

	tick := time.NewTicker(time.Duration(c.config.Metrics.Step) * time.Second)
	defer tick.Stop()

	logger.Infof(ctx, "workload %s metric report start", w.Meta.Appname)
	defer logger.Infof(ctx, "workload %s metric report stop", w.Meta.Appname)

	for {
		select {
		case <-tick.C:
			next, err := c.sample(w)
			if err != nil {
				logger.Error(ctx, err, "failed to sample")
				return
			}
			c.publish(ctx, client, prev, next)
			prev = next
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) sample(w *source.Workload) (*sample, error) {
	cg := &cgroup{path: w.CgroupPath}
	cpu, err := cg.cpu()
	if err != nil {
		return nil, err
	}
	mem, err := cg.mem()
	if err != nil {
		return nil, err
	}
	io, err := cg.io()
	if err != nil {
		return nil, err
	}
	net, err := c.netStats(w)
	if err != nil {
		return nil, err
	}
	host, err := hostCPUTimes(c.procRoot)
	if err != nil {
		return nil, err
	}
	return &sample{cpu: cpu, mem: mem, io: io, net: net, host: host}, nil
}

func (c *Collector) netStats(w *source.Workload) ([]netStat, error) {
	switch {
	case w.NetnsPID > 0:
		return netStatsFromProc(c.procRoot, w.NetnsPID)
	case w.HostIface != "":
		return netStatsFromIface(sysNetRoot, w.HostIface)
	default:
		return nil, nil
	}
}

func (c *Collector) publish(ctx context.Context, client *MetricsClient, prev, next *sample) {
	step := float64(c.config.Metrics.Step)

	deltaUsage := next.cpu.Usage - prev.cpu.Usage
	deltaUser := next.cpu.User - prev.cpu.User
	deltaSystem := next.cpu.System - prev.cpu.System

	client.CPUHostUsage(deltaUsage / (c.cpuCores * step))
	client.CPUHostSysUsage(ratio(deltaSystem, next.host.System-prev.host.System))
	client.CPUHostUserUsage(ratio(deltaUser, next.host.User-prev.host.User))

	client.CPUContainerUsage(deltaUsage / (cmp.Or(next.cpu.Limit, c.cpuCores) * step))
	client.CPUContainerSysUsage(ratio(deltaSystem, deltaUsage))
	client.CPUContainerUserUsage(ratio(deltaUser, deltaUsage))

	client.MemUsage(float64(next.mem.Current))
	client.MemMaxUsage(float64(next.mem.Peak))
	client.MemRss(float64(next.mem.Anon))
	if limit := cmp.Or(next.mem.Limit, c.memTotal); limit > 0 {
		client.MemPercent(float64(next.mem.Current) / float64(limit))
		client.MemRSSPercent(float64(next.mem.Anon) / float64(limit))
	}

	publishNet(client, prev.net, next.net, step)
	c.publishIO(ctx, client, prev.io, next.io, step)

	if err := client.Send(ctx); err != nil {
		log.WithFunc("collector.publish").Error(ctx, err, "send metrics failed")
	}
}

func (c *Collector) publishIO(ctx context.Context, client *MetricsClient, prev, next []ioStat, step float64) {
	before := make(map[string]ioStat, len(prev))
	for _, dev := range prev {
		if path, err := c.devicePath(dev.Major, dev.Minor); err == nil {
			before[path] = dev
		}
	}

	for _, dev := range next {
		path, err := c.devicePath(dev.Major, dev.Minor)
		if err != nil {
			log.WithFunc("collector.publishIO").Debugf(ctx, "no device node for %d:%d", dev.Major, dev.Minor)
			continue
		}
		client.IOServiceBytesRead(path, float64(dev.ReadBytes))
		client.IOServiceBytesWrite(path, float64(dev.WriteBytes))
		client.IOServicedRead(path, float64(dev.ReadIOs))
		client.IOServicedWrite(path, float64(dev.WriteIOs))

		old := before[path]
		client.IOServiceBytesReadPerSecond(path, float64(dev.ReadBytes-old.ReadBytes)/step)
		client.IOServiceBytesWritePerSecond(path, float64(dev.WriteBytes-old.WriteBytes)/step)
		client.IOServicedReadPerSecond(path, float64(dev.ReadIOs-old.ReadIOs)/step)
		client.IOServicedWritePerSecond(path, float64(dev.WriteIOs-old.WriteIOs)/step)
	}
}

// devicePath caches the /dev walk so a tick costs a map lookup per device.
func (c *Collector) devicePath(major, minor uint64) (string, error) {
	key := device{major: major, minor: minor}

	c.devicesMutex.Lock()
	defer c.devicesMutex.Unlock()
	if path, ok := c.devices[key]; ok {
		return path, nil
	}
	path, err := utils.GetDevicePath(major, minor)
	if err != nil {
		return "", err
	}
	c.devices[key] = path
	return path, nil
}

func publishNet(client *MetricsClient, prev, next []netStat, step float64) {
	before := make(map[string]netStat, len(prev))
	for _, nic := range prev {
		before[nic.Name] = nic
	}

	for _, nic := range next {
		old, ok := before[nic.Name]
		if !ok {
			continue
		}
		client.BytesSent(nic.Name, float64(nic.BytesSent-old.BytesSent)/step)
		client.BytesRecv(nic.Name, float64(nic.BytesRecv-old.BytesRecv)/step)
		client.PacketsSent(nic.Name, float64(nic.PacketsSent-old.PacketsSent)/step)
		client.PacketsRecv(nic.Name, float64(nic.PacketsRecv-old.PacketsRecv)/step)
		client.ErrIn(nic.Name, float64(nic.ErrIn-old.ErrIn)/step)
		client.ErrOut(nic.Name, float64(nic.ErrOut-old.ErrOut)/step)
		client.DropIn(nic.Name, float64(nic.DropIn-old.DropIn)/step)
		client.DropOut(nic.Name, float64(nic.DropOut-old.DropOut)/step)
	}
}

func ratio(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return part / whole
}
