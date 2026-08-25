package meta

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

const (
	// IDPattern is the shape of a workload id, so nothing else eru names on a node is taken for one.
	IDPattern = "[0-9a-f]{32}"

	KindProcess Kind = "process"
	KindVM      Kind = "vm"

	suffix = ".json"
)

var (
	errOtherKind = errors.New("meta file of another runtime")

	workloadID = regexp.MustCompile("^" + IDPattern + "$")
)

// Kind is the runtime the workload a meta file describes belongs to.
type Kind string

type logSource struct {
	JournalUnit       string `json:"journal_unit"`
	JournalIdentifier string `json:"journal_identifier"`
	ConsoleSocket     string `json:"console_socket"`
}

type healthCheck struct {
	TCPPorts []string `json:"tcp_ports"`
	HTTPPort string   `json:"http_port"`
	HTTPURL  string   `json:"http_url"`
	HTTPCode int      `json:"http_code"`
}

func (h *healthCheck) coreHealthCheck() *coretypes.HealthCheck {
	if h == nil {
		return nil
	}
	return &coretypes.HealthCheck{
		TCPPorts: h.TCPPorts,
		HTTPPort: h.HTTPPort,
		HTTPURL:  h.HTTPURL,
		HTTPCode: h.HTTPCode,
	}
}

// File is the metadata core writes next to a workload it created over ssh.
type File struct {
	ID          string            `json:"id"`
	Kind        Kind              `json:"kind"`
	Appname     string            `json:"appname"`
	Entrypoint  string            `json:"entrypoint"`
	Ident       string            `json:"ident"`
	Podname     string            `json:"podname"`
	Nodename    string            `json:"nodename"`
	CoreID      string            `json:"coreid"`
	Labels      map[string]string `json:"labels"`
	HealthCheck *healthCheck      `json:"healthcheck"`
	Publish     []string          `json:"publish"`
	Networks    map[string]string `json:"networks"`
	Cgroup      string            `json:"cgroup"`
	NetnsPID    int               `json:"netns_pid"`
	Iface       string            `json:"iface"`
	Log         logSource         `json:"log"`
}

// Workload renders the metadata as the workload every collector reads.
func (f *File) Workload(running bool) *source.Workload {
	return &source.Workload{
		ID: f.ID,
		Meta: source.Meta{
			Appname:     f.Appname,
			Entrypoint:  f.Entrypoint,
			Ident:       f.Ident,
			Podname:     f.Podname,
			Nodename:    f.Nodename,
			CoreID:      f.CoreID,
			Labels:      f.Labels,
			HealthCheck: f.HealthCheck.coreHealthCheck(),
			Publish:     f.Publish,
			Networks:    f.Networks,
		},
		Log: source.Log{
			JournalUnit:       f.Log.JournalUnit,
			JournalIdentifier: f.Log.JournalIdentifier,
			ConsoleSocket:     f.Log.ConsoleSocket,
		},
		CgroupPath:        f.Cgroup,
		NetnsPID:          f.NetnsPID,
		HostIface:         f.Iface,
		HostIfaceMirrored: f.Kind == KindVM,
		LocalIP:           cmp.Or(source.Addr(f.Networks), common.LocalIP),
		Running:           running,
	}
}

// IDFromFile returns the workload id a meta file's name carries.
func IDFromFile(name string) (string, bool) {
	return strings.CutSuffix(name, suffix)
}

// IsID reports whether a name is a workload id, so nothing else a node names is taken for a workload.
func IsID(name string) bool {
	return workloadID.MatchString(name)
}

func read(dir, ID string, kind Kind) (*File, error) {
	data, err := os.ReadFile(filepath.Join(dir, ID+suffix)) //nolint:gosec // the id comes from the meta dir listing or from an event
	if err != nil {
		return nil, err
	}
	f := &File{}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, err
	}
	if f.Kind != kind {
		return nil, fmt.Errorf("%w: meta file of %s describes a %q workload", errOtherKind, ID, f.Kind)
	}
	if !IsID(f.ID) {
		return nil, fmt.Errorf("meta file of %s carries %q, which is not a workload id", ID, f.ID)
	}
	if f.Log == (logSource{}) {
		return nil, fmt.Errorf("meta file of %s says nothing about where its logs are", ID)
	}
	return f, nil
}
