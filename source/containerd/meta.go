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

// ociSpec is the slice of the runtime spec the agent reads, so the rest of a multi-kilobyte spec is never allocated.
type ociSpec struct {
	Process *struct {
		Env []string `json:"env"`
	} `json:"process"`
	Linux *struct {
		Namespaces []ociNamespace `json:"namespaces"`
	} `json:"linux"`
}

type ociNamespace struct {
	Type string `json:"type"`
}

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
	oci := &ociSpec{}
	if err := json.Unmarshal(raw.GetValue(), oci); err != nil {
		return s, err
	}
	if oci.Process != nil {
		s.env = oci.Process.Env
	}
	// a container that shares the node's network has no network namespace of its own in the spec
	s.hostNetwork = oci.Linux != nil && !slices.ContainsFunc(oci.Linux.Namespaces, func(ns ociNamespace) bool {
		return ns.Type == string(specs.NetworkNamespace)
	})
	return s, nil
}

// workload maps the labels and runtime spec core wrote at create time onto one workload.
func (c *Containerd) workload(ctx context.Context, ID string, labels map[string]string, s spec) (*source.Workload, error) {
	appname, entrypoint, ident, err := coreutils.ParseWorkloadName(ID)
	if err != nil {
		return nil, err
	}

	podname, nodename := placement(s.env)
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
			Podname:     podname,
			Nodename:    nodename,
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

func placement(env []string) (podname, nodename string) {
	for _, e := range env {
		switch name, value, _ := strings.Cut(e, "="); name {
		case fieldPodName:
			podname = value
		case fieldNodeName:
			nodename = value
		}
	}
	return podname, nodename
}
