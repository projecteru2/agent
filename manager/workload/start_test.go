package workload

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/manager"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
)

func TestStartCollectingLeavesARunningSamplerAlone(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir()}

	m.startCollecting(t.Context(), w)
	first := m.collecting[w.ID]
	require.NotNil(t, first)

	for range 5 {
		m.startCollecting(t.Context(), w)
	}
	assert.Same(t, first, m.collecting[w.ID])
	m.stopCollecting(w.ID)
}

func TestStartCollectingRestartsASamplerThatReturned(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir()}
	ctx, cancel := context.WithCancel(t.Context())

	m.startCollecting(ctx, w)
	first := m.collecting[w.ID]
	require.NotNil(t, first)
	cancel()
	<-first.done

	m.startCollecting(t.Context(), w)
	assert.NotSame(t, first, m.collecting[w.ID])
	m.stopCollecting(w.ID)
}

func TestStartCollectingSkipsAWorkloadWithoutACgroup(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei"}

	m.startCollecting(t.Context(), w)
	assert.Empty(t, m.collecting)
}

func TestStartCollectingStartsAgainAfterTheWorkloadDied(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei", CgroupPath: t.TempDir()}

	m.startCollecting(t.Context(), w)
	first := m.collecting[w.ID]

	m.stopCollecting(w.ID)
	assert.Empty(t, m.collecting)

	m.startCollecting(t.Context(), w)
	assert.NotSame(t, first, m.collecting[w.ID])
}

func TestStartForwardingLeavesARunningForwardAlone(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei", Log: source.Log{JournalUnit: "eru-Rei.service"}}

	m.startForwarding(t.Context(), w)
	first := m.logTargets[w.ID]
	require.NotNil(t, first)

	for range 5 {
		m.startForwarding(t.Context(), w)
	}
	assert.Same(t, first, m.logTargets[w.ID])
	assert.Same(t, first, m.logTargets[w.Log.JournalUnit])
}

func TestStartForwardingForwardsAgainAfterTheWorkloadDied(t *testing.T) {
	m := managerForStart(t)
	w := &source.Workload{ID: "Rei", Log: source.Log{JournalUnit: "eru-Rei.service"}}

	m.startForwarding(t.Context(), w)
	first := m.logTargets[w.ID]

	m.stopForwarding(w.ID)
	assert.Empty(t, m.logTargets)

	m.startForwarding(t.Context(), w)
	assert.NotSame(t, first, m.logTargets[w.ID])
}

func TestStartForwardingSharesOneWriterPerTarget(t *testing.T) {
	m := managerForStart(t)
	rei := &source.Workload{ID: "Rei"}
	asuka := &source.Workload{ID: "Asuka"}

	m.startForwarding(t.Context(), rei)
	m.startForwarding(t.Context(), asuka)
	assert.Same(t, m.logTargets[rei.ID].writer, m.logTargets[asuka.ID].writer)
	assert.Len(t, m.writers, 1)

	m.stopForwarding(rei.ID)
	assert.Len(t, m.writers, 1)
	m.stopForwarding(asuka.ID)
	assert.Empty(t, m.writers)
}

func managerForStart(t *testing.T) *Manager {
	t.Helper()
	config := &types.Config{Metrics: types.MetricsConfig{Step: 10}}
	return NewManager(t.Context(), config, &manager.Clients{})
}
