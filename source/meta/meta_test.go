package meta

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
	vmWorkload      = "3c4d5e6f708192a3b4c5d6e7f8091a2b"
)

func TestReadRendersAProcessWorkload(t *testing.T) {
	f, err := processDir().Read(cniWorkload)
	require.NoError(t, err)

	w := f.Workload(true)
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

func TestReadRendersAVMWorkload(t *testing.T) {
	f, err := vmDir().Read(vmWorkload)
	require.NoError(t, err)

	w := f.Workload(true)
	assert.Equal(t, "/sys/fs/cgroup/cocoon.slice/cocoon-"+vmWorkload+".scope", w.CgroupPath)
	assert.Equal(t, "tap-"+vmWorkload, w.HostIface)
	assert.Zero(t, w.NetnsPID)
	assert.Equal(t, "10.0.0.9", w.LocalIP)
	assert.Equal(t, "/var/log/cocoon/ch/"+vmWorkload+"/console.log", w.Log.File)
	assert.Empty(t, w.Log.JournalUnit)
}

func TestReadOfAHostNetworkWorkload(t *testing.T) {
	f, err := processDir().Read(hostNetWorkload)
	require.NoError(t, err)

	w := f.Workload(false)
	assert.False(t, w.Running)
	assert.Equal(t, common.LocalIP, w.LocalIP)
	assert.Nil(t, w.Meta.HealthCheck)
}

func TestReadRejectsBadInput(t *testing.T) {
	_, err := processDir().Read("broken")
	assert.Error(t, err)

	_, err = processDir().Read("absent")
	assert.Error(t, err)
}

func TestReadRejectsAFileThatSaysNothingAboutLogs(t *testing.T) {
	_, err := processDir().Read(noLogWorkload)
	assert.ErrorContains(t, err, "says nothing about where its logs are")
}

func TestReadRejectsAFileWhoseIDIsNotAWorkloadID(t *testing.T) {
	_, err := processDir().Read("agent")
	assert.ErrorContains(t, err, "is not a workload id")
}

func TestReadRejectsAFileOfAnotherRuntime(t *testing.T) {
	_, err := processDir().Read(vmWorkload)
	assert.ErrorIs(t, err, errOtherKind)

	_, err = vmDir().Read(cniWorkload)
	assert.ErrorIs(t, err, errOtherKind)
}

func TestListYieldsOnlyTheDirsOwnKind(t *testing.T) {
	files, err := processDir().List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{cniWorkload, hostNetWorkload}, ids(files))

	files, err = vmDir().List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{vmWorkload}, ids(files))
}

func TestIDFromFile(t *testing.T) {
	ID, ok := IDFromFile(cniWorkload + ".json")
	assert.True(t, ok)
	assert.Equal(t, cniWorkload, ID)

	_, ok = IDFromFile(cniWorkload + ".json.tmp")
	assert.False(t, ok)
}

func processDir() *Dir {
	return &Dir{path: "testdata", kind: KindProcess}
}

func vmDir() *Dir {
	return &Dir{path: "testdata", kind: KindVM}
}

func ids(files []*File) []string {
	found := make([]string, 0, len(files))
	for _, f := range files {
		found = append(found, f.ID)
	}
	return found
}
