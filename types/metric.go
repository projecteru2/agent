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

type NetStats struct {
	Inbytes    uint64
	Inpackets  uint64
	Inerrs     uint64
	Indrop     uint64
	Outbytes   uint64
	Outpackets uint64
	Outerrs    uint64
	Outdrop    uint64
}
