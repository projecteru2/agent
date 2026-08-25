package ocihook

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cni "github.com/containerd/go-cni"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

func TestReadStateAtCreateRuntimeIsAnAdd(t *testing.T) {
	s, err := readState(strings.NewReader(`{
		"ociVersion": "1.0.2",
		"id": "myapp_web_EAXPcM",
		"status": "creating",
		"pid": 4242,
		"bundle": "/run/containerd/io.containerd.runtime.v2.task/eru/myapp_web_EAXPcM",
		"annotations": {"eru.namespace": "staging"}
	}`))
	require.NoError(t, err)

	assert.Equal(t, "myapp_web_EAXPcM", s.ID)
	assert.True(t, s.adding())
	assert.Equal(t, "/proc/4242/ns/net", s.netns())
	assert.Equal(t, "staging", s.namespace())
}

func TestReadStateAtPoststopIsADelete(t *testing.T) {
	tests := []struct {
		name  string
		state string
		netns string
	}{
		{
			name:  "runc reports the container stopped once the process is gone",
			state: `{"id": "myapp_web_EAXPcM", "status": "stopped", "pid": 0}`,
		},
		{
			name:  "a stopped container whose pid the runtime still reports",
			state: `{"id": "myapp_web_EAXPcM", "status": "stopped", "pid": 4242}`,
			netns: "/proc/4242/ns/net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := readState(strings.NewReader(tt.state))
			require.NoError(t, err)
			assert.False(t, s.adding())
			assert.Equal(t, tt.netns, s.netns())
			assert.Equal(t, common.ContainerdNamespace, s.namespace())
		})
	}
}

func TestReadStateOfALiveContainerIsAnAdd(t *testing.T) {
	for _, status := range []string{"creating", "created", "running"} {
		t.Run(status, func(t *testing.T) {
			s, err := readState(strings.NewReader(`{"id": "myapp_web_EAXPcM", "status": "` + status + `", "pid": 4242}`))
			require.NoError(t, err)
			assert.True(t, s.adding())
		})
	}
}

func TestReadStateRejectsBadInput(t *testing.T) {
	_, err := readState(strings.NewReader("not json"))
	assert.ErrorContains(t, err, "read the oci state")

	_, err = readState(strings.NewReader(`{"status": "created", "pid": 4242}`))
	assert.ErrorContains(t, err, "no container id")
}

func TestConfListPicksTheNetworkByName(t *testing.T) {
	dir := t.TempDir()
	writeConfList(t, dir, "10-other.conflist", "other")
	writeConfList(t, dir, "20-eru-cni.conflist", "eru-cni")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "30-broken.conflist"), []byte("not json"), 0o600))

	conf, err := confList(dir, "eru-cni")
	require.NoError(t, err)
	assert.Contains(t, string(conf), `"name": "eru-cni"`)
}

func TestConfListWithoutTheNetwork(t *testing.T) {
	dir := t.TempDir()
	writeConfList(t, dir, "10-other.conflist", "other")

	_, err := confList(dir, "eru-cni")
	assert.ErrorContains(t, err, "no cni network named eru-cni")
}

func TestAddressOfTakesTheFirstIPv4(t *testing.T) {
	result := &cni.Result{Interfaces: map[string]*cni.Config{
		"eth0": {IPConfigs: []*cni.IPConfig{
			{IP: net.ParseIP("fd00::5")},
			{IP: net.ParseIP("10.0.0.5")},
		}},
		"a-host-side-veth": {},
	}}

	assert.Equal(t, "10.0.0.5", addressOf(result))
}

func TestAddressOfAResultWithoutAnAddress(t *testing.T) {
	assert.Empty(t, addressOf(&cni.Result{Interfaces: map[string]*cni.Config{"eth0": {}}}))
}

func writeConfList(t *testing.T, dir, file, name string) {
	t.Helper()
	conf := `{"cniVersion": "1.0.0", "name": "` + name + `", "plugins": [{"type": "bridge"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(conf), 0o600))
}
