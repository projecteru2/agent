package containerd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/utils"
)

const (
	fieldPodName  = "ERU_POD"
	fieldNodeName = "ERU_NODE_NAME"

	networkLabelPrefix = "eru.network."
)

// workload maps the labels and environment core wrote at create time onto one workload.
func workload(ctx context.Context, ID string, labels map[string]string, env []string) (*source.Workload, error) {
	appname, entrypoint, ident, err := utils.GetAppInfo(ID)
	if err != nil {
		return nil, err
	}

	vars := normalizeEnv(env)
	meta := coreutils.DecodeMetaInLabel(ctx, labels)
	nets := networks(labels)

	return &source.Workload{
		ID: ID,
		Meta: source.Meta{
			Appname:     appname,
			Entrypoint:  entrypoint,
			Ident:       ident,
			Podname:     vars[fieldPodName],
			Nodename:    vars[fieldNodeName],
			CoreID:      labels[cluster.LabelCoreID],
			Labels:      labels,
			HealthCheck: meta.HealthCheck,
			Publish:     meta.Publish,
			Networks:    nets,
		},
		Log:     source.Log{JournalIdentifier: common.JournalIdentifier},
		LocalIP: source.LocalIP(nets),
	}, nil
}

func specEnv(spec typeurl.Any) ([]string, error) {
	if spec == nil {
		return nil, nil
	}
	// containerd stores the runtime spec as json, so the typeurl value is the spec itself
	s := &specs.Spec{}
	if err := json.Unmarshal(spec.GetValue(), s); err != nil {
		return nil, err
	}
	if s.Process == nil {
		return nil, nil
	}
	return s.Process.Env, nil
}

// networks reads the addresses the oci hook wrote back after it ran cni.
func networks(labels map[string]string) map[string]string {
	nets := map[string]string{}
	for key, addr := range labels {
		if name, ok := strings.CutPrefix(key, networkLabelPrefix); ok {
			nets[name] = addr
		}
	}
	return nets
}

func normalizeEnv(env []string) map[string]string {
	vars := make(map[string]string, len(env))
	for _, e := range env {
		name, value, _ := strings.Cut(e, "=")
		vars[name] = value
	}
	return vars
}
