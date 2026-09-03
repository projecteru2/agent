package collector

import (
	"cmp"
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

const (
	sysNetRoot = "/sys/class/net"

	// hostCacheTTL is under the smallest useful metrics step, so a tick still sees a fresh read
	hostCacheTTL = time.Second
)

type refreshFunc func() *source.Workload

type device struct {
	major uint64
	minor uint64
}

type sample struct {
	at     time.Time
	hostAt time.Time

	cpu  cpuStat
	mem  memStat
	io   []ioStat
	net  []netStat
	host hostCPU
}

// unsupported names the gauges this node's kernel cannot answer, so they are never exported.
func (s *sample) unsupported() []string {
	if s.mem.HasPeak {
		return nil
	}
	return []string{metricMemMaxUsage}
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

	hostMutex sync.Mutex
	hostAt    time.Time
	hostTimes hostCPU
	hostErr   error
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

// Collect samples one workload every metrics step until ctx is canceled; a step that fails is retried on the next one.
func (c *Collector) Collect(ctx context.Context, w *source.Workload, refresh refreshFunc) {
	logger := log.WithFunc("collector.Collect").WithField("ID", w.ID)
	if w.CgroupPath == "" {
		logger.Debug(ctx, "workload has no cgroup, not sampling it")
		return
	}
	defer removeMetricsClient(w.ID)

	tick := time.NewTicker(time.Duration(c.config.Metrics.Step) * time.Second)
	defer tick.Stop()

	logger.Infof(ctx, "workload %s metric report start", w.Meta.Appname)
	defer logger.Infof(ctx, "workload %s metric report stop", w.Meta.Appname)

	var (
		client *MetricsClient
		prev   *sample
		broken bool
	)
	step := func() {
		next, err := c.sample(ctx, w)
		if err != nil {
			if !broken {
				logger.Error(ctx, err, "failed to sample, retrying every step until it works")
				broken = true
			}
			if fresh := refresh(); fresh != nil {
				w = fresh
			}
			prev = nil
			return
		}
		broken = false
		if client == nil {
			client = c.clientFor(w, next)
		}
		if prev != nil {
			c.publish(ctx, client, prev, next)
		}
		prev = next
	}

	step()
	for {
		select {
		case <-tick.C:
			step()
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) clientFor(w *source.Workload, first *sample) *MetricsClient {
	return NewMetricsClient(c.transfers.Get(w.ID, 0), c.hostname, w, first.unsupported())
}

func (c *Collector) sample(ctx context.Context, w *source.Workload) (*sample, error) {
	cg := &cgroup{path: w.CgroupPath}
	at := time.Now()
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
	host, hostAt, err := c.host()
	if err != nil {
		return nil, err
	}
	return &sample{at: at, hostAt: hostAt, cpu: cpu, mem: mem, io: io, net: c.netStats(ctx, w), host: host}, nil
}

// host reads /proc/stat at most once per cache ttl: it is the same file for every workload here.
func (c *Collector) host() (hostCPU, time.Time, error) {
	c.hostMutex.Lock()
	defer c.hostMutex.Unlock()

	if time.Since(c.hostAt) >= hostCacheTTL {
		c.hostTimes, c.hostErr = hostCPUTimes(c.procRoot)
		c.hostAt = time.Now()
	}
	return c.hostTimes, c.hostAt, c.hostErr
}

func (c *Collector) netStats(ctx context.Context, w *source.Workload) []netStat {
	var stats []netStat
	var err error
	switch {
	case w.NetnsPID > 0:
		stats, err = netStatsFromProc(c.procRoot, w.NetnsPID, w.HostIface, w.HostIfaceMirrored)
	case w.HostIfaceMirrored:
		return nil
	case w.HostIface != "":
		stats, err = netStatsFromIface(sysNetRoot, w.HostIface)
	default:
		return nil
	}
	if err != nil {
		log.WithFunc("collector.netStats").WithField("ID", w.ID).Debugf(ctx, "no network counters this step: %v", err)
		return nil
	}
	return stats
}

func (c *Collector) publish(ctx context.Context, client *MetricsClient, prev, next *sample) {
	elapsed := next.at.Sub(prev.at).Seconds()
	if elapsed <= 0 || next.cpu.Usage < prev.cpu.Usage {
		return
	}
	hostElapsed := next.hostAt.Sub(prev.hostAt).Seconds()

	deltaUsage := next.cpu.Usage - prev.cpu.Usage
	deltaUser := next.cpu.User - prev.cpu.User
	deltaSystem := next.cpu.System - prev.cpu.System
	hostSystem := ratio(next.host.System-prev.host.System, hostElapsed)
	hostUser := ratio(next.host.User-prev.host.User, hostElapsed)

	client.CPUHostUsage(deltaUsage / (c.cpuCores * elapsed))
	client.CPUHostSysUsage(ratio(deltaSystem/elapsed, hostSystem))
	client.CPUHostUserUsage(ratio(deltaUser/elapsed, hostUser))

	client.CPUContainerUsage(deltaUsage / (cmp.Or(next.cpu.Limit, c.cpuCores) * elapsed))
	client.CPUContainerSysUsage(ratio(deltaSystem, deltaUsage))
	client.CPUContainerUserUsage(ratio(deltaUser, deltaUsage))

	client.MemUsage(float64(next.mem.Current))
	client.MemMaxUsage(float64(next.mem.Peak))
	client.MemRss(float64(next.mem.Anon))
	if limit := cmp.Or(next.mem.Limit, c.memTotal); limit > 0 {
		client.MemPercent(float64(next.mem.Current) / float64(limit))
		client.MemRSSPercent(float64(next.mem.Anon) / float64(limit))
	}

	publishNet(client, prev.net, next.net, elapsed)
	c.publishIO(ctx, client, prev.io, next.io, elapsed)

	if err := client.Send(ctx); err != nil {
		log.WithFunc("collector.publish").Error(ctx, err, "send metrics failed")
	}
}

func (c *Collector) publishIO(ctx context.Context, client *MetricsClient, prev, next []ioStat, elapsed float64) {
	before := make(map[device]ioStat, len(prev))
	for _, dev := range prev {
		before[device{major: dev.Major, minor: dev.Minor}] = dev
	}

	for _, dev := range next {
		key := device{major: dev.Major, minor: dev.Minor}
		path, ok := c.devicePath(key)
		if !ok {
			log.WithFunc("collector.publishIO").Debugf(ctx, "no device node for %d:%d", dev.Major, dev.Minor)
			continue
		}
		client.IOServiceBytesRead(path, float64(dev.ReadBytes))
		client.IOServiceBytesWrite(path, float64(dev.WriteBytes))
		client.IOServicedRead(path, float64(dev.ReadIOs))
		client.IOServicedWrite(path, float64(dev.WriteIOs))

		old, ok := before[key]
		if !ok || dev.rewound(old) {
			continue
		}
		client.IOServiceBytesReadPerSecond(path, float64(dev.ReadBytes-old.ReadBytes)/elapsed)
		client.IOServiceBytesWritePerSecond(path, float64(dev.WriteBytes-old.WriteBytes)/elapsed)
		client.IOServicedReadPerSecond(path, float64(dev.ReadIOs-old.ReadIOs)/elapsed)
		client.IOServicedWritePerSecond(path, float64(dev.WriteIOs-old.WriteIOs)/elapsed)
	}
}

// devicePath caches the /dev walk so a tick costs a map lookup per device.
func (c *Collector) devicePath(key device) (string, bool) {
	c.devicesMutex.Lock()
	path, cached := c.devices[key]
	c.devicesMutex.Unlock()
	if cached {
		return path, path != ""
	}

	path, err := utils.GetDevicePath(key.major, key.minor)
	if err != nil && !errors.Is(err, common.ErrDevNotFound) {
		return "", false
	}
	c.devicesMutex.Lock()
	c.devices[key] = path
	c.devicesMutex.Unlock()
	return path, path != ""
}

func publishNet(client *MetricsClient, prev, next []netStat, elapsed float64) {
	before := make(map[string]netStat, len(prev))
	for _, nic := range prev {
		before[nic.Name] = nic
	}

	for _, nic := range next {
		old, ok := before[nic.Name]
		if !ok || nic.rewound(old) {
			continue
		}
		client.BytesSent(nic.Name, float64(nic.BytesSent-old.BytesSent)/elapsed)
		client.BytesRecv(nic.Name, float64(nic.BytesRecv-old.BytesRecv)/elapsed)
		client.PacketsSent(nic.Name, float64(nic.PacketsSent-old.PacketsSent)/elapsed)
		client.PacketsRecv(nic.Name, float64(nic.PacketsRecv-old.PacketsRecv)/elapsed)
		client.ErrIn(nic.Name, float64(nic.ErrIn-old.ErrIn)/elapsed)
		client.ErrOut(nic.Name, float64(nic.ErrOut-old.ErrOut)/elapsed)
		client.DropIn(nic.Name, float64(nic.DropIn-old.DropIn)/elapsed)
		client.DropOut(nic.Name, float64(nic.DropOut-old.DropOut)/elapsed)
	}
}

func ratio(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return part / whole
}
