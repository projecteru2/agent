package source

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

var ErrUnknownWorkload = errors.New("no runtime on this node knows this workload")

// Source is one runtime's view of the workloads this node runs.
type Source interface {
	List(ctx context.Context) ([]*Workload, error)
	Get(ctx context.Context, ID string) (*Workload, error)
	Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error)
	Alive(ctx context.Context) bool
}

// Meta is the workload metadata core wrote when it created the workload.
type Meta struct {
	Appname     string
	Entrypoint  string
	Ident       string
	Podname     string
	Nodename    string
	CoreID      string
	Labels      map[string]string
	HealthCheck *coretypes.HealthCheck
	Publish     []string
	Networks    map[string]string
}

// Log locates a workload's output: a journal unit, a journal identifier field or a file.
type Log struct {
	JournalUnit       string
	JournalIdentifier string
	File              string
}

// Workload is everything a source knows about one workload.
type Workload struct {
	ID   string
	Meta Meta
	Log  Log

	CgroupPath string
	NetnsPID   int
	HostIface  string

	LocalIP string
	Running bool
}

// LogFields returns the extra fields every log record of this workload carries.
func (w *Workload) LogFields() map[string]string {
	fields := map[string]string{
		"podname":  w.Meta.Podname,
		"nodename": w.Meta.Nodename,
		"coreid":   w.Meta.CoreID,
	}
	for name, addr := range w.Meta.Networks {
		fields[fmt.Sprintf("networks_%s", name)] = addr
	}
	return fields
}

// LocalIP returns the workload's own address, or localhost when it shares the host network.
func LocalIP(networks map[string]string) string {
	for _, name := range slices.Sorted(maps.Keys(networks)) {
		if addr := networks[name]; addr != "" {
			return addr
		}
	}
	return common.LocalIP
}
