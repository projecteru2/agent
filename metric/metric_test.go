package metric

import (
	"bufio"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	cpuContent = `user 8671
system 6682
`
	usageContent      = `12128256`
	maxUsageContent   = `14303232`
	memoryStatContent = `cache 8900608
rss 3219456
rss_huge 0
mapped_file 4489216
swap 0
pgpgin 14139
pgpgout 11180
pgfault 14107
pgmajfault 79
inactive_anon 1470464
active_anon 1748992
inactive_file 8519680
active_file 380928
unevictable 0
hierarchical_memory_limit 268435456
hierarchical_memsw_limit 268435456
total_cache 8900608
total_rss 3219456
total_rss_huge 0
total_mapped_file 4489216
total_swap 0
total_pgpgin 14139
total_pgpgout 11180
total_pgfault 14107
total_pgmajfault 79
total_inactive_anon 1470464
total_active_anon 1748992
total_inactive_file 8519680
total_active_file 380928
total_unevictable 0
`
	netStatContent = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
 cali0: 159148863 1883419    0    0    0     0          0         0 128700272 1255613    0    0    0     0       0          0
	lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
`
	statContent = `cpu  31635996 613 19127585 1203972226 136478 0 86513 466312 0 0
cpu0 15860599 405 9621006 601912014 61123 0 47932 235176 0 0
cpu1 15775397 207 9506579 602060212 75355 0 38581 231135 0 0
intr 9863181098 121 168 0 0 674 0 3 0 1 0 0 34 15 0 0 0 0 0 0 0 0 0 0 0 0 69093439 1834 0 3 0 3309965 0 3969032 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 27666717600
btime 1495881218
processes 13030892
procs_running 1
procs_blocked 0
softirq 2123549386 1 1010265009 5859 224999994 0 0 212 545351625 0 342926686
`
)

// write content to temp file
func mockNewStats() *Stats {
	cpuPath, _ := ioutil.TempFile(os.TempDir(), "cpuPath-")
	defer cpuPath.Close()
	cpuPath.WriteString(cpuContent)

	memoryUsagePath, _ := ioutil.TempFile(os.TempDir(), "memoryUsagePath-")
	defer memoryUsagePath.Close()
	memoryUsagePath.WriteString(usageContent)

	memoryMaxUsagePath, _ := ioutil.TempFile(os.TempDir(), "memoryMaxUsagePath-")
	defer memoryMaxUsagePath.Close()
	memoryMaxUsagePath.WriteString(maxUsageContent)

	memoryDetailPath, _ := ioutil.TempFile(os.TempDir(), "memoryDetailPath-")
	defer memoryDetailPath.Close()
	memoryDetailPath.WriteString(memoryStatContent)

	networkStatsPath, _ := ioutil.TempFile(os.TempDir(), "networkStatsPath-")
	defer networkStatsPath.Close()
	networkStatsPath.WriteString(netStatContent)

	statPath, _ := ioutil.TempFile(os.TempDir(), "statPath-")
	defer statPath.Close()
	statPath.WriteString(statContent)

	return &Stats{
		cid:                "ae0099a44fcf814a0a15b5f3391c8f05bc8b851265861e229aae4c040d611419",
		pid:                666,
		bufReader:          bufio.NewReaderSize(nil, 128),
		cpuPath:            cpuPath.Name(),
		memoryUsagePath:    memoryUsagePath.Name(),
		memoryMaxUsagePath: memoryMaxUsagePath.Name(),
		memoryDetailPath:   memoryDetailPath.Name(),
		networkStatsPath:   networkStatsPath.Name(),
		statFilePath:       statPath.Name(),
	}
}

func removeMockFiles(mockStats *Stats) {
	os.Remove(mockStats.cpuPath)
	os.Remove(mockStats.memoryDetailPath)
	os.Remove(mockStats.memoryMaxUsagePath)
	os.Remove(mockStats.memoryUsagePath)
	os.Remove(mockStats.networkStatsPath)
	os.Remove(mockStats.statFilePath)
}

func TestGetCPUStats(t *testing.T) {
	mockStats := mockNewStats()
	defer removeMockFiles(mockStats)

	cpuStats, err := mockStats.GetCPUStats()
	assert.NoError(t, err)
	assert.Equal(t, uint64(8671), cpuStats.UsageInUserMode)
	assert.Equal(t, uint64(6682), cpuStats.UsageInSystemMode)

}

func TestGetMemoryStats(t *testing.T) {
	mockStats := mockNewStats()
	defer removeMockFiles(mockStats)

	memoryStats, err := mockStats.GetMemoryStats()
	assert.NoError(t, err)
	assert.Equal(t, uint64(14303232), memoryStats.MaxUsage)
	assert.Equal(t, uint64(12128256), memoryStats.Usage)
	assert.Equal(t, uint64(268435456), memoryStats.Detail["hierarchical_memory_limit"])
}

func TestGetNetworkStats(t *testing.T) {
	mockStats := mockNewStats()
	defer removeMockFiles(mockStats)

	networkStats, err := mockStats.GetNetworkStats()
	assert.NoError(t, err)
	assert.Equal(t, uint64(159148863), networkStats["cali0.inbytes"])
}

func TestGetTotalJiffies(t *testing.T) {
	mockStats := mockNewStats()
	defer removeMockFiles(mockStats)

	totalJiffies, tsReadingTotalJiffies, err := mockStats.GetTotalJiffies()
	assert.NoError(t, err)
	t.Log(totalJiffies)
	t.Log(tsReadingTotalJiffies)
}
