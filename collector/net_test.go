package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetStatsFromProc(t *testing.T) {
	stats, err := netStatsFromProc("testdata/proc", 1234)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	assert.Equal(t, netStat{Name: "lo", BytesRecv: 100, PacketsRecv: 1, BytesSent: 100, PacketsSent: 1}, stats[0])
	assert.Equal(t, netStat{
		Name:        "eth0",
		BytesRecv:   2000,
		PacketsRecv: 20,
		ErrIn:       1,
		DropIn:      2,
		BytesSent:   3000,
		PacketsSent: 30,
		ErrOut:      3,
		DropOut:     4,
	}, stats[1])
}

func TestNetStatsFromProcFailsForAWorkloadThatIsGone(t *testing.T) {
	_, err := netStatsFromProc("testdata/proc", 4321)
	assert.Error(t, err)
}

func TestNetStatsFromIface(t *testing.T) {
	stats, err := netStatsFromIface("testdata/sys", "eth0", false)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, netStat{
		Name:        "eth0",
		BytesRecv:   111,
		BytesSent:   222,
		PacketsRecv: 3,
		PacketsSent: 4,
		ErrIn:       5,
		ErrOut:      6,
		DropIn:      7,
		DropOut:     8,
	}, stats[0])
}

func TestNetStatsFromIfaceMirrorsAHostSideTap(t *testing.T) {
	stats, err := netStatsFromIface("testdata/sys", "eth0", true)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, netStat{
		Name:        "eth0",
		BytesRecv:   222,
		BytesSent:   111,
		PacketsRecv: 4,
		PacketsSent: 3,
		ErrIn:       6,
		ErrOut:      5,
		DropIn:      8,
		DropOut:     7,
	}, stats[0])
}
