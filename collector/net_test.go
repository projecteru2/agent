package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const vmTap = "tapXBBR7U22-0"

func TestNetStatsFromProc(t *testing.T) {
	stats, err := netStatsFromProc("testdata/proc", 1234, "", false)
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
	_, err := netStatsFromProc("testdata/proc", 4321, "", false)
	assert.Error(t, err)
}

func TestNetStatsFromProcNarrowsToTheWorkloadsIface(t *testing.T) {
	stats, err := netStatsFromProc("testdata/proc", 2345, vmTap, false)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, netStat{
		Name:        vmTap,
		BytesRecv:   5000,
		PacketsRecv: 50,
		ErrIn:       1,
		DropIn:      2,
		BytesSent:   7000,
		PacketsSent: 70,
		ErrOut:      3,
		DropOut:     4,
	}, stats[0])
}

func TestNetStatsFromProcMirrorsAVMTapInItsNetns(t *testing.T) {
	stats, err := netStatsFromProc("testdata/proc", 2345, vmTap, true)
	require.NoError(t, err)
	require.Len(t, stats, 1)

	assert.Equal(t, netStat{
		Name:        vmTap,
		BytesRecv:   7000,
		PacketsRecv: 70,
		ErrIn:       3,
		DropIn:      4,
		BytesSent:   5000,
		PacketsSent: 50,
		ErrOut:      1,
		DropOut:     2,
	}, stats[0])
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
