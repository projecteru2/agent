package systemd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/source"
)

const (
	metaSuffix = ".json"

	workloadIDPattern = "[0-9a-f]{32}"
)

var (
	// workloadUnit matches a workload's transient unit and nothing else eru puts on a node under the
	// same prefix, in particular not the agent's own eru-agent.service or a core host's eru-core.service.
	workloadUnit = regexp.MustCompile("^" + unitPrefix + "(" + workloadIDPattern + ")" + regexp.QuoteMeta(unitSuffix) + "$")

	workloadID = regexp.MustCompile("^" + workloadIDPattern + "$")
)

type logSource struct {
	JournalUnit       string `json:"journal_unit"`
	JournalIdentifier string `json:"journal_identifier"`
	File              string `json:"file"`
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

// meta is the file core writes next to a workload it created over ssh.
type meta struct {
	ID          string            `json:"id"`
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

func readMeta(dir, ID string) (*meta, error) {
	data, err := os.ReadFile(filepath.Join(dir, ID+metaSuffix)) //nolint:gosec // the id comes from the meta dir listing or from an event
	if err != nil {
		return nil, err
	}
	m := &meta{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if !workloadID.MatchString(m.ID) {
		return nil, fmt.Errorf("meta file of %s carries %q, which is not a workload id", ID, m.ID)
	}
	if m.Log == (logSource{}) {
		return nil, fmt.Errorf("meta file of %s says nothing about where its logs are", ID)
	}
	return m, nil
}

func (m *meta) workload(running bool) *source.Workload {
	return &source.Workload{
		ID: m.ID,
		Meta: source.Meta{
			Appname:     m.Appname,
			Entrypoint:  m.Entrypoint,
			Ident:       m.Ident,
			Podname:     m.Podname,
			Nodename:    m.Nodename,
			CoreID:      m.CoreID,
			Labels:      m.Labels,
			HealthCheck: m.HealthCheck.coreHealthCheck(),
			Publish:     m.Publish,
			Networks:    m.Networks,
		},
		Log: source.Log{
			JournalUnit:       m.Log.JournalUnit,
			JournalIdentifier: m.Log.JournalIdentifier,
			File:              m.Log.File,
		},
		CgroupPath: m.Cgroup,
		NetnsPID:   m.NetnsPID,
		HostIface:  m.Iface,
		LocalIP:    source.LocalIP(m.Networks),
		Running:    running,
	}
}

func unitOf(ID string) string {
	return unitPrefix + ID + unitSuffix
}

func workloadIDFromFile(name string) (string, bool) {
	return strings.CutSuffix(name, metaSuffix)
}

func workloadIDFromUnit(name string) (string, bool) {
	match := workloadUnit.FindStringSubmatch(name)
	if match == nil {
		return "", false
	}
	return match[1], true
}
