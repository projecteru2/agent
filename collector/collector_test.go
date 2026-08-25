package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

	restoreCgroupFile(t, dir, "memory.current")
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

	first, err := c.host()
	require.NoError(t, err)
	at := c.hostAt

	second, err := c.host()
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, at, c.hostAt)
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
		body, err := os.ReadFile(filepath.Join("testdata/cgroup", entry.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, entry.Name()), body, 0o600))
	}
	return dir
}

func restoreCgroupFile(t *testing.T, dir, name string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata/cgroup", name))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o600))
}
