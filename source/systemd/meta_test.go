package systemd

import (
	"testing"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

const (
	cniWorkload     = "0f9c1a2b3d4e5f60718293a4b5c6d7e8"
	hostNetWorkload = "1a2b3c4d5e6f708192a3b4c5d6e7f809"
	noLogWorkload   = "2b3c4d5e6f708192a3b4c5d6e7f8091a"
)

func TestReadMeta(t *testing.T) {
	m, err := readMeta("testdata", cniWorkload)
	require.NoError(t, err)

	w := m.workload(true)
	assert.Equal(t, cniWorkload, w.ID)
	assert.True(t, w.Running)
	assert.Equal(t, "/sys/fs/cgroup/eru.slice/eru-"+cniWorkload+".service", w.CgroupPath)
	assert.Equal(t, "eru-"+cniWorkload+".service", w.Log.JournalUnit)
	assert.Equal(t, "10.0.0.5", w.LocalIP)
	assert.Equal(t, source.Meta{
		Appname:     "myapp",
		Entrypoint:  "web",
		Ident:       "EAXPcM",
		Podname:     "prod",
		Nodename:    "node-1",
		CoreID:      "core-1",
		Labels:      map[string]string{"eru.build": "1"},
		HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{"80"}, HTTPPort: "80", HTTPURL: "/healthz", HTTPCode: 200},
		Publish:     []string{"80"},
		Networks:    map[string]string{"eru-cni": "10.0.0.5"},
	}, w.Meta)
}

func TestReadMetaOfAHostNetworkWorkload(t *testing.T) {
	m, err := readMeta("testdata", hostNetWorkload)
	require.NoError(t, err)

	w := m.workload(false)
	assert.False(t, w.Running)
	assert.Equal(t, common.LocalIP, w.LocalIP)
	assert.Nil(t, w.Meta.HealthCheck)
	assert.False(t, needsNetns(w))
}

func TestReadMetaRejectsBadInput(t *testing.T) {
	_, err := readMeta("testdata", "broken")
	assert.Error(t, err)

	_, err = readMeta("testdata", "absent")
	assert.Error(t, err)
}

func TestReadMetaRejectsAFileThatSaysNothingAboutLogs(t *testing.T) {
	_, err := readMeta("testdata", noLogWorkload)
	assert.ErrorContains(t, err, "says nothing about where its logs are")
}

func TestReadMetaRejectsAFileWhoseIDIsNotAWorkloadID(t *testing.T) {
	_, err := readMeta("testdata", "agent")
	assert.ErrorContains(t, err, "is not a workload id")
}

func TestNeedsNetnsOnlyForARunningWorkloadWithItsOwnNetwork(t *testing.T) {
	m, err := readMeta("testdata", cniWorkload)
	require.NoError(t, err)

	w := m.workload(true)
	assert.True(t, needsNetns(w))

	w.NetnsPID = 42
	assert.False(t, needsNetns(w))

	w.NetnsPID = 0
	w.HostIface = "tap-" + cniWorkload
	assert.False(t, needsNetns(w))

	assert.False(t, needsNetns(m.workload(false)))
}

func TestUnitOf(t *testing.T) {
	assert.Equal(t, "eru-"+cniWorkload+".service", unitOf(cniWorkload))
}

func TestWorkloadIDFromUnit(t *testing.T) {
	tests := []struct {
		name string
		unit string
		want string
	}{
		{"a workload unit", "eru-" + cniWorkload + ".service", cniWorkload},
		{"the agent's own unit", "eru-agent.service", ""},
		{"a core host's unit", "eru-core.service", ""},
		{"an unrelated unit", "sshd.service", ""},
		{"a workload's scope rather than its service", "eru-" + cniWorkload + ".scope", ""},
		{"an id that is too short", "eru-0f9c1a2b.service", ""},
		{"an id that is too long", "eru-" + cniWorkload + "ff.service", ""},
		{"an id that is not hex", "eru-zzzc1a2b3d4e5f60718293a4b5c6d7e8.service", ""},
		{"an id in upper case", "eru-0F9C1A2B3D4E5F60718293A4B5C6D7E8.service", ""},
		{"a bare prefix", "eru-.service", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ID, ok := workloadIDFromUnit(tt.unit)
			assert.Equal(t, tt.want != "", ok)
			assert.Equal(t, tt.want, ID)
		})
	}
}

func TestWorkloadIDFromFile(t *testing.T) {
	ID, ok := workloadIDFromFile(cniWorkload + ".json")
	assert.True(t, ok)
	assert.Equal(t, cniWorkload, ID)

	_, ok = workloadIDFromFile(cniWorkload + ".json.tmp")
	assert.False(t, ok)
}
