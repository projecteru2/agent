package containerd

import (
	"testing"

	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projecteru2/core/cluster"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

func TestWorkloadFromLabelsAndEnv(t *testing.T) {
	w, err := workload(t.Context(), "myapp_web_EAXPcM", eruLabels(t), []string{"ERU_POD=prod", "ERU_NODE_NAME=node-1", "APP_NAME=myapp"})
	require.NoError(t, err)

	assert.Equal(t, "myapp_web_EAXPcM", w.ID)
	assert.Equal(t, "10.0.0.5", w.LocalIP)
	assert.Equal(t, source.Log{JournalIdentifier: common.JournalIdentifier}, w.Log)
	assert.Equal(t, source.Meta{
		Appname:     "myapp",
		Entrypoint:  "web",
		Ident:       "EAXPcM",
		Podname:     "prod",
		Nodename:    "node-1",
		CoreID:      "core-1",
		Labels:      eruLabels(t),
		HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{"80"}, HTTPPort: "80", HTTPURL: "/healthz", HTTPCode: 200},
		Publish:     []string{"80"},
		Networks:    map[string]string{"eru-cni": "10.0.0.5"},
	}, w.Meta)
}

func TestWorkloadOnTheHostNetworkProbesLocalhost(t *testing.T) {
	w, err := workload(t.Context(), "myapp_web_EAXPcM", map[string]string{cluster.ERUMark: "1"}, nil)
	require.NoError(t, err)

	assert.Equal(t, common.LocalIP, w.LocalIP)
	assert.Empty(t, w.Meta.Networks)
	assert.Nil(t, w.Meta.HealthCheck)
}

func TestWorkloadRejectsAnIDThatIsNotAWorkloadName(t *testing.T) {
	_, err := workload(t.Context(), "web_EAXPcM", map[string]string{cluster.ERUMark: "1"}, nil)
	assert.ErrorContains(t, err, "invalid workload name")
}

func TestNetworksReadsOnlyTheHookLabels(t *testing.T) {
	nets := networks(map[string]string{
		"eru.network.eru-cni": "10.0.0.5",
		"eru.network.second":  "10.0.1.5",
		"eru.nodename":        "node-1",
		"eru.networkish":      "not-an-address",
	})

	assert.Equal(t, map[string]string{"eru-cni": "10.0.0.5", "second": "10.0.1.5"}, nets)
}

func TestSpecEnv(t *testing.T) {
	spec, err := typeurl.MarshalAny(&specs.Spec{Process: &specs.Process{Env: []string{"ERU_POD=prod"}}})
	require.NoError(t, err)

	env, err := specEnv(spec)
	require.NoError(t, err)
	assert.Equal(t, []string{"ERU_POD=prod"}, env)
}

func TestSpecEnvOfAContainerWithoutAProcess(t *testing.T) {
	env, err := specEnv(nil)
	require.NoError(t, err)
	assert.Nil(t, env)

	spec, err := typeurl.MarshalAny(&specs.Spec{})
	require.NoError(t, err)
	env, err = specEnv(spec)
	require.NoError(t, err)
	assert.Nil(t, env)
}

func eruLabels(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		cluster.ERUMark:     "1",
		cluster.LabelCoreID: "core-1",
		cluster.LabelMeta: coreutils.EncodeMetaInLabel(t.Context(), &coretypes.LabelMeta{
			Publish:     []string{"80"},
			HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{"80"}, HTTPPort: "80", HTTPURL: "/healthz", HTTPCode: 200},
		}),
		cluster.LabelNodeName:                 "node-1",
		common.NetworkLabelPrefix + "eru-cni": "10.0.0.5",
	}
}
