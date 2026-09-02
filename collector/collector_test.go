package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
)

const (
	sampleStep    = 1
	sampleSettle  = 1500 * time.Millisecond
	sampleTimeout = 8 * time.Second
	samplePoll    = 50 * time.Millisecond
)

func TestSampleDropsMemMaxUsageWithoutMemoryPeak(t *testing.T) {
	assert.Nil(t, (&sample{mem: memStat{HasPeak: true}}).unsupported())
	assert.Equal(t, []string{"mem_max_usage"}, (&sample{}).unsupported())
}

func TestMetricsClientSkipsAnUnsupportedGauge(t *testing.T) {
	w := &source.Workload{ID: "without-memory-peak", Meta: source.Meta{Appname: "app", Entrypoint: "web"}}
	client := NewMetricsClient("", "node", w, []string{"mem_max_usage"})
	defer removeMetricsClient(w.ID)

	assert.NotContains(t, client.gauges, "mem_max_usage")
	assert.Contains(t, client.gauges, "mem_usage")

	client.MemMaxUsage(42)
	assert.NotContains(t, client.data, "mem_max_usage")
}

func TestCollectRetriesAfterAFailedSample(t *testing.T) {
	dir := cgroupWithout(t, "memory.current")
	w := &source.Workload{ID: "gains-its-controller-late", Meta: source.Meta{Appname: "app", Entrypoint: "web"}, CgroupPath: dir}

	c := New(t.Context(), &types.Config{Metrics: types.MetricsConfig{Step: sampleStep}})
	c.procRoot = "testdata/proc"
	go c.Collect(t.Context(), w, noRefresh)
	t.Cleanup(func() { removeMetricsClient(w.ID) })

	assert.Never(t, func() bool { return sampling(w.ID) }, sampleSettle, samplePoll)

	copyCgroupFile(t, dir, "memory.current")
	assert.Eventually(t, func() bool { return sampling(w.ID) }, sampleTimeout, samplePoll)
}

func TestCollectAdoptsTheMetadataItReReadsAfterAFailure(t *testing.T) {
	w := &source.Workload{
		ID:         "moved-to-a-new-scope",
		Meta:       source.Meta{Appname: "app", Entrypoint: "web"},
		CgroupPath: cgroupWithout(t, "memory.current"),
	}
	fresh := &source.Workload{ID: w.ID, Meta: w.Meta, CgroupPath: cgroupWithout(t, "")}

	c := New(t.Context(), &types.Config{Metrics: types.MetricsConfig{Step: sampleStep}})
	c.procRoot = "testdata/proc"
	go c.Collect(t.Context(), w, func() *source.Workload { return fresh })
	t.Cleanup(func() { removeMetricsClient(w.ID) })

	assert.Eventually(t, func() bool { return sampling(w.ID) }, sampleTimeout, samplePoll)
}

func TestMetricsClientScrubsEruInternals(t *testing.T) {
	w := &source.Workload{ID: "scrub", Meta: source.Meta{
		Appname:    "my.app",
		Entrypoint: "web",
		Labels:     map[string]string{"eru.network.calico": "10.0.0.5", "ERU": "1", "team": "infra"},
	}}
	client := NewMetricsClient("127.0.0.1:8125", "node", w, nil)
	defer removeMetricsClient(w.ID)

	assert.Contains(t, client.prefix, "my-app.web")
	descs := make(chan *prometheus.Desc, 1)
	client.collectors[0].Describe(descs)
	desc := (<-descs).String()
	assert.Contains(t, desc, "team=infra")
	assert.NotContains(t, desc, "eru.network")
}

func TestPublishSkipsAStepAcrossACounterReset(t *testing.T) {
	w := &source.Workload{ID: "restarted-in-place", Meta: source.Meta{Appname: "app", Entrypoint: "web"}}
	client := NewMetricsClient("", "node", w, nil)
	defer removeMetricsClient(w.ID)
	c := &Collector{cpuCores: 1}
	now := time.Now()

	c.publish(t.Context(), client, &sample{at: now, cpu: cpuStat{Usage: 100}}, &sample{at: now.Add(time.Second), cpu: cpuStat{Usage: 1}})
	assert.Zero(t, testutil.ToFloat64(client.gauges["cpu_host_usage"].plain))

	c.publish(t.Context(), client, &sample{at: now, cpu: cpuStat{Usage: 1}}, &sample{at: now.Add(time.Second), cpu: cpuStat{Usage: 2}})
	assert.Positive(t, testutil.ToFloat64(client.gauges["cpu_host_usage"].plain))
}

func TestPublishDividesByTheMeasuredWindow(t *testing.T) {
	w := &source.Workload{ID: "late-tick", Meta: source.Meta{Appname: "app", Entrypoint: "web"}}
	client := NewMetricsClient("", "node", w, nil)
	defer removeMetricsClient(w.ID)
	c := &Collector{cpuCores: 1}
	now := time.Now()
	prev := &sample{at: now, hostAt: now, cpu: cpuStat{Usage: 1, System: 1}, host: hostCPU{System: 4}}
	next := &sample{at: now.Add(2 * time.Second), hostAt: now, cpu: cpuStat{Usage: 2, System: 2}, host: hostCPU{System: 4}}

	c.publish(t.Context(), client, prev, next)
	assert.InDelta(t, 0.5, testutil.ToFloat64(client.gauges["cpu_host_usage"].plain), 1e-9)
	assert.Zero(t, testutil.ToFloat64(client.gauges["cpu_host_sys_usage"].plain))

	next.hostAt = now.Add(time.Second)
	next.host.System = 6
	c.publish(t.Context(), client, prev, next)
	assert.InDelta(t, 0.25, testutil.ToFloat64(client.gauges["cpu_host_sys_usage"].plain), 1e-9)
}

func TestRewoundCatchesACounterTheBytesOutran(t *testing.T) {
	oldNic := netStat{BytesSent: 100, PacketsSent: 1000, DropIn: 5}
	assert.False(t, (&netStat{BytesSent: 100, PacketsSent: 1000, DropIn: 5}).rewound(oldNic))
	assert.True(t, (&netStat{BytesSent: 200, PacketsSent: 1000}).rewound(oldNic))
	assert.True(t, (&netStat{BytesSent: 200, PacketsSent: 17, DropIn: 5}).rewound(oldNic))
	assert.False(t, (&netStat{BytesSent: 200, PacketsSent: 1200, DropIn: 5}).rewound(oldNic))

	oldDev := ioStat{ReadBytes: 100, ReadIOs: 10}
	assert.True(t, ioStat{ReadBytes: 200, ReadIOs: 3}.rewound(oldDev))
	assert.False(t, ioStat{ReadBytes: 200, ReadIOs: 12}.rewound(oldDev))
}

func TestPublishIOSkipsTheRateOfADeviceItJustMet(t *testing.T) {
	w := &source.Workload{ID: "first-write-to-a-new-device", Meta: source.Meta{Appname: "app", Entrypoint: "web"}}
	client := NewMetricsClient("", "node", w, nil)
	defer removeMetricsClient(w.ID)
	c := &Collector{devices: map[device]string{{major: 8, minor: 16}: "sdb"}}

	c.publishIO(t.Context(), client, nil, []ioStat{{Major: 8, Minor: 16, WriteBytes: 1 << 30}}, sampleStep)

	write := client.gauges["io_service_bytes_write"].vector.WithLabelValues("sdb")
	rate := client.gauges["io_service_bytes_write_per_second"].vector.WithLabelValues("sdb")
	assert.Positive(t, testutil.ToFloat64(write))
	assert.Zero(t, testutil.ToFloat64(rate))
}

func TestNetStatsSkipTheHostLookupForAVMBeforeItsFirstStart(t *testing.T) {
	c := &Collector{procRoot: "testdata/proc"}
	w := &source.Workload{ID: "not-started-yet", HostIface: "tapXBBR7U22-0", HostIfaceMirrored: true}

	assert.Nil(t, c.netStats(t.Context(), w))
}

func TestNetStatsSurviveANamespaceThatIsGone(t *testing.T) {
	c := &Collector{procRoot: "testdata/proc"}
	w := &source.Workload{ID: "netns-gone", NetnsPID: 4321}

	assert.Nil(t, c.netStats(t.Context(), w))
}

func TestSampleKeepsTheCgroupGaugesWhenTheNetnsIsGone(t *testing.T) {
	c := &Collector{procRoot: "testdata/proc"}
	w := &source.Workload{ID: "netns-gone", CgroupPath: "testdata/cgroup", NetnsPID: 4321}

	got, err := c.sample(t.Context(), w)
	require.NoError(t, err)
	assert.Nil(t, got.net)
	assert.NotZero(t, got.mem.Current)
}

func TestCollectorCachesTheNodeCPUTimes(t *testing.T) {
	c := &Collector{config: &types.Config{}, procRoot: "testdata/proc"}

	first, at, err := c.host()
	require.NoError(t, err)

	second, again, err := c.host()
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, at, again)
}

func noRefresh() *source.Workload {
	return nil
}

func sampling(ID string) bool {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	_, ok := clients[ID]
	return ok
}

func cgroupWithout(t *testing.T, missing string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir("testdata/cgroup")
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.Name() == missing {
			continue
		}
		copyCgroupFile(t, dir, entry.Name())
	}
	return dir
}

func copyCgroupFile(t *testing.T, dir, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata/cgroup", name))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o600))
}
