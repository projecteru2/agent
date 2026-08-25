package collector

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	unlimited = "max"

	microsecond = 1e6
)

type cpuStat struct {
	Usage  float64
	User   float64
	System float64
	Limit  float64
}

type memStat struct {
	Current uint64
	Peak    uint64
	Anon    uint64
	Limit   uint64
}

type ioStat struct {
	Major      uint64
	Minor      uint64
	ReadBytes  uint64
	WriteBytes uint64
	ReadIOs    uint64
	WriteIOs   uint64
}

type cgroup struct {
	path string
}

func (c *cgroup) cpu() (cpuStat, error) {
	values, err := readKeyed(filepath.Join(c.path, "cpu.stat"))
	if err != nil {
		return cpuStat{}, err
	}
	limit, err := c.cpuLimit()
	if err != nil {
		return cpuStat{}, err
	}
	return cpuStat{
		Usage:  float64(values["usage_usec"]) / microsecond,
		User:   float64(values["user_usec"]) / microsecond,
		System: float64(values["system_usec"]) / microsecond,
		Limit:  limit,
	}, nil
}

func (c *cgroup) mem() (memStat, error) {
	current, err := readUint(filepath.Join(c.path, "memory.current"))
	if err != nil {
		return memStat{}, err
	}
	values, err := readKeyed(filepath.Join(c.path, "memory.stat"))
	if err != nil {
		return memStat{}, err
	}
	// memory.peak needs linux 5.19, memory.max is absent without the memory controller
	peak, err := readUintOptional(filepath.Join(c.path, "memory.peak"))
	if err != nil {
		return memStat{}, err
	}
	limit, err := readUintOptional(filepath.Join(c.path, "memory.max"))
	if err != nil {
		return memStat{}, err
	}
	return memStat{Current: current, Peak: peak, Anon: values["anon"], Limit: limit}, nil
}

func (c *cgroup) io() ([]ioStat, error) {
	lines, err := readLines(filepath.Join(c.path, "io.stat"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	stats := make([]ioStat, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		major, minor, ok := parseDevice(fields)
		if !ok {
			continue
		}
		stats = append(stats, ioCounters(ioStat{Major: major, Minor: minor}, fields[1:]))
	}
	return stats, nil
}

func (c *cgroup) cpuLimit() (float64, error) {
	data, err := os.ReadFile(filepath.Join(c.path, "cpu.max")) //nolint:gosec // the path comes from the runtime source, never from a request
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	quota, period, ok := strings.Cut(strings.TrimSpace(string(data)), " ")
	if !ok || quota == unlimited {
		return 0, nil
	}
	allowed, err := strconv.ParseFloat(quota, 64)
	if err != nil {
		return 0, err
	}
	window, err := strconv.ParseFloat(period, 64)
	if err != nil {
		return 0, err
	}
	if window == 0 {
		return 0, nil
	}
	return allowed / window, nil
}

func ioCounters(stat ioStat, fields []string) ioStat {
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "rbytes":
			stat.ReadBytes = n
		case "wbytes":
			stat.WriteBytes = n
		case "rios":
			stat.ReadIOs = n
		case "wios":
			stat.WriteIOs = n
		}
	}
	return stat
}

func parseDevice(fields []string) (major, minor uint64, ok bool) {
	if len(fields) < 2 {
		return 0, 0, false
	}
	majorField, minorField, found := strings.Cut(fields[0], ":")
	if !found {
		return 0, 0, false
	}
	major, err := strconv.ParseUint(majorField, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.ParseUint(minorField, 10, 64)
	return major, minor, err == nil
}

func readKeyed(path string) (map[string]uint64, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	values := make(map[string]uint64, len(lines))
	for _, line := range lines {
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if n, err := strconv.ParseUint(value, 10, 64); err == nil {
			values[key] = n
		}
	}
	return values, nil
}

func readUint(path string) (uint64, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the runtime source, never from a request
	if err != nil {
		return 0, err
	}
	field := strings.TrimSpace(string(data))
	if field == unlimited {
		return 0, nil
	}
	return strconv.ParseUint(field, 10, 64)
}

func readUintOptional(path string) (uint64, error) {
	value, err := readUint(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return value, err
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // the path comes from the runtime source, never from a request
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
