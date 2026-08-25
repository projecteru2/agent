package collector

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type netStat struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	ErrIn       uint64
	ErrOut      uint64
	DropIn      uint64
	DropOut     uint64
}

// netStatsFromProc reads every interface of the netns pid lives in.
func netStatsFromProc(procRoot string, pid int) ([]netStat, error) {
	lines, err := readLines(filepath.Join(procRoot, strconv.Itoa(pid), "net", "dev"))
	if err != nil {
		return nil, err
	}
	if len(lines) < 2 {
		return nil, fmt.Errorf("net/dev of %d has no header", pid)
	}

	stats := make([]netStat, 0, len(lines)-2)
	for _, line := range lines[2:] {
		name, counters, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) < 12 {
			continue
		}
		stats = append(stats, netStat{
			Name:        strings.TrimSpace(name),
			BytesRecv:   parseUint(fields[0]),
			PacketsRecv: parseUint(fields[1]),
			ErrIn:       parseUint(fields[2]),
			DropIn:      parseUint(fields[3]),
			BytesSent:   parseUint(fields[8]),
			PacketsSent: parseUint(fields[9]),
			ErrOut:      parseUint(fields[10]),
			DropOut:     parseUint(fields[11]),
		})
	}
	return stats, nil
}

// netStatsFromIface reads the counters of one host interface from sysfs.
func netStatsFromIface(sysRoot, iface string) ([]netStat, error) {
	dir := filepath.Join(sysRoot, iface, "statistics")
	stat := netStat{Name: iface}
	for _, counter := range []struct {
		name  string
		field *uint64
	}{
		{"rx_bytes", &stat.BytesRecv},
		{"tx_bytes", &stat.BytesSent},
		{"rx_packets", &stat.PacketsRecv},
		{"tx_packets", &stat.PacketsSent},
		{"rx_errors", &stat.ErrIn},
		{"tx_errors", &stat.ErrOut},
		{"rx_dropped", &stat.DropIn},
		{"tx_dropped", &stat.DropOut},
	} {
		value, err := readUint(filepath.Join(dir, counter.name))
		if err != nil {
			return nil, err
		}
		*counter.field = value
	}
	return []netStat{stat}, nil
}

func parseUint(field string) uint64 {
	value, _ := strconv.ParseUint(field, 10, 64)
	return value
}
