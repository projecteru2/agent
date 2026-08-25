package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCgroupCPU(t *testing.T) {
	stat, err := (&cgroup{path: "testdata/cgroup"}).cpu()
	require.NoError(t, err)

	assert.Equal(t, 1.5, stat.Usage)
	assert.Equal(t, 1.0, stat.User)
	assert.Equal(t, 0.5, stat.System)
	assert.Equal(t, 2.0, stat.Limit)
}

func TestCgroupCPUReportsNoLimitWhenTheQuotaIsMax(t *testing.T) {
	stat, err := (&cgroup{path: "testdata/unlimited"}).cpu()
	require.NoError(t, err)

	assert.Equal(t, 7.0, stat.Usage)
	assert.Zero(t, stat.Limit)
}

func TestCgroupMem(t *testing.T) {
	stat, err := (&cgroup{path: "testdata/cgroup"}).mem()
	require.NoError(t, err)

	assert.Equal(t, uint64(104857600), stat.Current)
	assert.Equal(t, uint64(209715200), stat.Peak)
	assert.Equal(t, uint64(52428800), stat.Anon)
	assert.Equal(t, uint64(1073741824), stat.Limit)
}

func TestCgroupMemToleratesAnAbsentPeakAndAnUnlimitedMax(t *testing.T) {
	stat, err := (&cgroup{path: "testdata/unlimited"}).mem()
	require.NoError(t, err)

	assert.Equal(t, uint64(2048), stat.Current)
	assert.Equal(t, uint64(1024), stat.Anon)
	assert.Zero(t, stat.Peak)
	assert.Zero(t, stat.Limit)
}

func TestCgroupIO(t *testing.T) {
	stats, err := (&cgroup{path: "testdata/cgroup"}).io()
	require.NoError(t, err)
	require.Len(t, stats, 2)

	assert.Equal(t, ioStat{Major: 8, ReadBytes: 1024, WriteBytes: 2048, ReadIOs: 10, WriteIOs: 20}, stats[0])
	assert.Equal(t, ioStat{Major: 253, Minor: 1, ReadBytes: 4096, WriteBytes: 8192, ReadIOs: 40, WriteIOs: 80}, stats[1])
}

func TestCgroupIOIsEmptyWithoutTheIOController(t *testing.T) {
	stats, err := (&cgroup{path: "testdata/unlimited"}).io()
	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestCgroupFailsWhenTheWorkloadIsGone(t *testing.T) {
	gone := &cgroup{path: "testdata/no-such-workload"}

	_, err := gone.cpu()
	assert.Error(t, err)
	_, err = gone.mem()
	assert.Error(t, err)
}
