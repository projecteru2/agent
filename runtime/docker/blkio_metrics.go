package docker

import (
	enginecontainer "github.com/docker/docker/api/types/container"

	"github.com/projecteru2/agent/utils"
)

const (
	ReadOp  = "Read"
	WriteOp = "Write"
)

type BlkIOMetrics struct {
	IOServiceBytesReadRecursive  []*BlkIOEntry
	IOServiceBytesWriteRecursive []*BlkIOEntry
	IOServicedReadRecusive       []*BlkIOEntry
	IOServicedWriteRecusive      []*BlkIOEntry
}

type BlkIOEntry struct {
	Dev   string
	Value uint64
}

func fromEngineBlkioStats(raw *enginecontainer.BlkioStats) (*BlkIOMetrics, error) {
	blkioMetrics := &BlkIOMetrics{}
	for _, entry := range raw.IoServiceBytesRecursive {
		devPath, err := utils.GetDevicePath(entry.Major, entry.Minor)
		if err != nil {
			return nil, err
		}
		switch entry.Op {
		case ReadOp:
			blkioMetrics.IOServiceBytesReadRecursive = append(blkioMetrics.IOServiceBytesReadRecursive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		case WriteOp:
			blkioMetrics.IOServiceBytesWriteRecursive = append(blkioMetrics.IOServiceBytesWriteRecursive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		}
	}
	for _, entry := range raw.IoServicedRecursive {
		devPath, err := utils.GetDevicePath(entry.Major, entry.Minor)
		if err != nil {
			return nil, err
		}
		switch entry.Op {
		case ReadOp:
			blkioMetrics.IOServicedReadRecusive = append(blkioMetrics.IOServicedReadRecusive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		case WriteOp:
			blkioMetrics.IOServicedWriteRecusive = append(blkioMetrics.IOServicedWriteRecusive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		}
	}
	return blkioMetrics, nil
}

// getBlkIOMetricsDifference returns new-old per device, counting a missing device as 0.
func getBlkIOMetricsDifference(old, new *BlkIOMetrics) (diff *BlkIOMetrics) {
	return &BlkIOMetrics{
		IOServiceBytesReadRecursive:  getGroupDifference(old.IOServiceBytesReadRecursive, new.IOServiceBytesReadRecursive),
		IOServiceBytesWriteRecursive: getGroupDifference(old.IOServiceBytesWriteRecursive, new.IOServiceBytesWriteRecursive),
		IOServicedReadRecusive:       getGroupDifference(old.IOServicedReadRecusive, new.IOServicedReadRecusive),
		IOServicedWriteRecusive:      getGroupDifference(old.IOServicedWriteRecusive, new.IOServicedWriteRecusive),
	}
}

func getGroupDifference(old, new []*BlkIOEntry) (diff []*BlkIOEntry) {
	lookup := func(dev string, entryList []*BlkIOEntry) uint64 {
		for _, entry := range entryList {
			if entry.Dev == dev {
				return entry.Value
			}
		}
		return 0
	}
	for _, entry := range new {
		diffEntry := &BlkIOEntry{
			Dev:   entry.Dev,
			Value: entry.Value - lookup(entry.Dev, old),
		}
		diff = append(diff, diffEntry)
	}
	return diff
}
