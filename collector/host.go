package collector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	clockTick = 100

	kilobyte = 1024
)

type hostCPU struct {
	User   float64
	System float64
}

// hostCPUTimes reads the node-wide user and system time, in seconds.
func hostCPUTimes(procRoot string) (hostCPU, error) {
	f, err := os.Open(filepath.Join(procRoot, "stat")) //nolint:gosec // procRoot is the configured proc mount
	if err != nil {
		return hostCPU{}, err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[0] != "cpu" {
			continue
		}
		return hostCPU{
			User:   float64(parseUint(fields[1])) / clockTick,
			System: float64(parseUint(fields[3])) / clockTick,
		}, nil
	}
	return hostCPU{}, fmt.Errorf("no aggregate cpu line in %s/stat", procRoot)
}

// hostMemTotal reads the node's total memory, in bytes.
func hostMemTotal(procRoot string) (uint64, error) {
	lines, err := readLines(filepath.Join(procRoot, "meminfo"))
	if err != nil {
		return 0, err
	}
	for _, line := range lines {
		value, ok := strings.CutPrefix(line, "MemTotal:")
		if !ok {
			continue
		}
		total, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), " kB"), 10, 64)
		if err != nil {
			return 0, err
		}
		return total * kilobyte, nil
	}
	return 0, fmt.Errorf("no MemTotal in %s/meminfo", procRoot)
}
