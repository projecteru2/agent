package systemd

import (
	"testing"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

func TestReadMeta(t *testing.T) {
	m, err := readMeta("testdata", "abc123")
	require.NoError(t, err)

	w := m.workload(true)
	assert.Equal(t, "abc123", w.ID)
	assert.True(t, w.Running)
	assert.Equal(t, "/sys/fs/cgroup/eru.slice/eru-abc123.service", w.CgroupPath)
	assert.Equal(t, "eru-abc123.service", w.Log.JournalUnit)
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
	m, err := readMeta("testdata", "hostnet")
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
	_, err := readMeta("testdata", "nolog")
	assert.ErrorContains(t, err, "says nothing about where its logs are")
}

func TestNeedsNetnsOnlyForARunningWorkloadWithItsOwnNetwork(t *testing.T) {
	m, err := readMeta("testdata", "abc123")
	require.NoError(t, err)

	w := m.workload(true)
	assert.True(t, needsNetns(w))

	w.NetnsPID = 42
	assert.False(t, needsNetns(w))

	w.NetnsPID = 0
	w.HostIface = "tap-abc123"
	assert.False(t, needsNetns(w))

	assert.False(t, needsNetns(m.workload(false)))
}

func TestUnitNaming(t *testing.T) {
	assert.Equal(t, "eru-abc123.service", unitOf("abc123"))

	ID, ok := workloadIDFromUnit("eru-abc123.service")
	assert.True(t, ok)
	assert.Equal(t, "abc123", ID)

	_, ok = workloadIDFromUnit("sshd.service")
	assert.False(t, ok)

	_, ok = workloadIDFromUnit("eru-abc123.scope")
	assert.False(t, ok)
}

func TestWorkloadIDFromFile(t *testing.T) {
	ID, ok := workloadIDFromFile("abc123.json")
	assert.True(t, ok)
	assert.Equal(t, "abc123", ID)

	_, ok = workloadIDFromFile("abc123.json.tmp")
	assert.False(t, ok)
}
