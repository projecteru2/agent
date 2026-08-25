package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostCPUTimes(t *testing.T) {
	times, err := hostCPUTimes("testdata/proc")
	require.NoError(t, err)

	assert.Equal(t, 1.0, times.User)
	assert.Equal(t, 3.0, times.System)
}

func TestHostMemTotal(t *testing.T) {
	total, err := hostMemTotal("testdata/proc")
	require.NoError(t, err)
	assert.Equal(t, uint64(16777216), total)
}

func TestHostReadsFailWithoutProcfs(t *testing.T) {
	_, err := hostCPUTimes("testdata/no-procfs")
	assert.Error(t, err)

	_, err = hostMemTotal("testdata/no-procfs")
	assert.Error(t, err)
}
