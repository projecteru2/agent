package containerd

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

const (
	fieldPodName  = "ERU_POD"
	fieldNodeName = "ERU_NODE_NAME"

	hostNetwork = "host"
)

// spec is what the container's runtime spec says about how core configured it.
type spec struct {
	env         []string
	hostNetwork bool
}

func readSpec(raw typeurl.Any) (spec, error) {
	s := spec{}
	if raw == nil {
		return s, nil
	}
	// containerd stores the runtime spec as json, so the typeurl value is the spec itself
	oci := &specs.Spec{}
	if err := json.Unmarshal(raw.GetValue(), oci); err != nil {
		return s, err
	}
	if oci.Process != nil {
		s.env = oci.Process.Env
	}
	// a container that shares the node's network has no network namespace of its own in the spec
	s.hostNetwork = oci.Linux != nil && !slices.ContainsFunc(oci.Linux.Namespaces, func(ns specs.LinuxNamespace) bool {
		return ns.Type == specs.NetworkNamespace
	})
	return s, nil
}

// workload maps the labels and runtime spec core wrote at create time onto one workload.
func (c *Containerd) workload(ctx context.Context, ID string, labels map[string]string, s spec) (*source.Workload, error) {
	appname, entrypoint, ident, err := coreutils.ParseWorkloadName(ID)
	if err != nil {
		return nil, err
	}

	vars := normalizeEnv(s.env)
	meta := coreutils.DecodeMetaInLabel(ctx, labels)
	nets := networks(labels)

	// core's engines all report the node's own address for a host network workload
	localIP := source.Addr(nets)
	if len(nets) == 0 && s.hostNetwork {
		nets, localIP = map[string]string{hostNetwork: c.nodeIP}, common.LocalIP
	}

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
			Networks:    nets,
		},
		LocalIP: localIP,
	}, nil
}

// networks reads the addresses the oci hook wrote back after it ran cni.
func networks(labels map[string]string) map[string]string {
	nets := map[string]string{}
	for key, addr := range labels {
		if name, ok := strings.CutPrefix(key, common.NetworkLabelPrefix); ok {
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
