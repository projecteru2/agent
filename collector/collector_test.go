package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
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
