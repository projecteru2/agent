package collector

import (
	"net"
	"testing"
	"time"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/source"
)

func TestProbeCallsAWorkloadWithoutAHealthCheckHealthy(t *testing.T) {
	w := &source.Workload{ID: "no-health-check", LocalIP: "127.0.0.1"}
	assert.True(t, Probe(t.Context(), w, time.Second))
}

func TestProbeAcceptsAnOpenTCPPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	w := &source.Workload{
		ID:      "open-port",
		LocalIP: "127.0.0.1",
		Meta:    source.Meta{HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{port}}},
	}
	assert.True(t, Probe(t.Context(), w, time.Second))
}

func TestProbeRejectsAClosedTCPPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, listener.Close())

	w := &source.Workload{
		ID:      "closed-port",
		LocalIP: "127.0.0.1",
		Meta:    source.Meta{HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{port}}},
	}
	assert.False(t, Probe(t.Context(), w, time.Second))
}
