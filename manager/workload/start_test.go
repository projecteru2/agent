package workload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
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
	w := &source.Workload{ID: "Rei"}

	m.startCollecting(t.Context(), w)
	first := m.collecting[w.ID]
	require.NotNil(t, first)
	<-first.done

	w.CgroupPath = t.TempDir()
	m.startCollecting(t.Context(), w)
	assert.NotSame(t, first, m.collecting[w.ID])
	m.stopCollecting(w.ID)
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

func managerForStart(t *testing.T) *Manager {
	t.Helper()
	config := &types.Config{Metrics: types.MetricsConfig{Step: 10}}

	return &Manager{
		config:     config,
		collector:  collector.New(t.Context(), config),
		forwards:   utils.NewHashBackends(nil),
		collecting: map[string]*collectTask{},
		logTargets: map[string]*logTarget{},
	}
}
