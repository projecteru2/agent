package types

type CPUStats struct {
	UsageInUserMode   uint64
	UsageInSystemMode uint64
}

type MemoryStats struct {
	Usage    uint64
	MaxUsage uint64
	Detail   map[string]uint64
}
