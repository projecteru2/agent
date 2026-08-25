package docker

import (
	enginecontainer "github.com/moby/moby/api/types/container"

	"github.com/projecteru2/agent/utils"
)

const (
	ReadOp  = "Read"
	WriteOp = "Write"
)

type BlkIOMetrics struct {
	IOServiceBytesReadRecursive  []*BlkIOEntry
	IOServiceBytesWriteRecursive []*BlkIOEntry
	IOServicedReadRecursive      []*BlkIOEntry
	IOServicedWriteRecursive     []*BlkIOEntry
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
			blkioMetrics.IOServicedReadRecursive = append(blkioMetrics.IOServicedReadRecursive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		case WriteOp:
			blkioMetrics.IOServicedWriteRecursive = append(blkioMetrics.IOServicedWriteRecursive, &BlkIOEntry{Dev: devPath, Value: entry.Value})
		}
	}
	return blkioMetrics, nil
}

func getBlkIOMetricsDifference(old, new *BlkIOMetrics) (diff *BlkIOMetrics) {
	return &BlkIOMetrics{
		IOServiceBytesReadRecursive:  getGroupDifference(old.IOServiceBytesReadRecursive, new.IOServiceBytesReadRecursive),
		IOServiceBytesWriteRecursive: getGroupDifference(old.IOServiceBytesWriteRecursive, new.IOServiceBytesWriteRecursive),
		IOServicedReadRecursive:      getGroupDifference(old.IOServicedReadRecursive, new.IOServicedReadRecursive),
		IOServicedWriteRecursive:     getGroupDifference(old.IOServicedWriteRecursive, new.IOServicedWriteRecursive),
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
