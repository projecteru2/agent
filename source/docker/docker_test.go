package docker

import (
	"net/netip"
	"testing"

	enginecontainer "github.com/moby/moby/api/types/container"
	enginenetwork "github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
)

func TestWorkloadNetworksPicksTheFirstNameInOrder(t *testing.T) {
	d := &Docker{nodeIP: "10.0.0.1"}

	localIP, networks := d.workloadNetworks(t.Context(), inspectResponse("zeta", "1.1.1.1", "alpha", "2.2.2.2"))
	assert.Equal(t, "2.2.2.2", localIP)
	assert.Equal(t, map[string]string{"alpha": "2.2.2.2"}, networks)
}

func TestWorkloadNetworksReportsTheNodeIPInHostMode(t *testing.T) {
	d := &Docker{nodeIP: "10.0.0.1"}

	localIP, networks := d.workloadNetworks(t.Context(), inspectResponse("host", ""))
	assert.Equal(t, common.LocalIP, localIP)
	assert.Equal(t, map[string]string{"host": "10.0.0.1"}, networks)
}

func TestWorkloadNetworksHandlesAWorkloadWithoutNetworks(t *testing.T) {
	d := &Docker{nodeIP: "10.0.0.1"}

	localIP, networks := d.workloadNetworks(t.Context(), inspectResponse())
	assert.Empty(t, localIP)
	assert.Empty(t, networks)
}

func TestWorkloadNetworksFallsBackToTheNetnsWhenTheEndpointHasNoAddress(t *testing.T) {
	d := &Docker{nodeIP: "10.0.0.1"}

	localIP, networks := d.workloadNetworks(t.Context(), inspectResponse("bridge", ""))
	assert.Empty(t, localIP)
	assert.Equal(t, map[string]string{"bridge": ""}, networks)
}

func inspectResponse(nameAddrPairs ...string) enginecontainer.InspectResponse {
	endpoints := map[string]*enginenetwork.EndpointSettings{}
	for i := 0; i < len(nameAddrPairs); i += 2 {
		addr, _ := netip.ParseAddr(nameAddrPairs[i+1])
		endpoints[nameAddrPairs[i]] = &enginenetwork.EndpointSettings{IPAddress: addr}
	}

	return enginecontainer.InspectResponse{
		ID:              "not-a-running-workload",
		NetworkSettings: &enginecontainer.NetworkSettings{Networks: endpoints},
	}
}
