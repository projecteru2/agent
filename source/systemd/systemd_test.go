package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

const workloadID = "0f9c1a2b3d4e5f60718293a4b5c6d7e8"

func TestActionForIgnoresTheTransitionalStates(t *testing.T) {
	tests := map[string]string{
		stateActive:    common.StatusStart,
		stateInactive:  common.StatusDie,
		stateFailed:    common.StatusDie,
		"activating":   "",
		"deactivating": "",
		"reloading":    "",
	}

	for state, want := range tests {
		t.Run(state, func(t *testing.T) {
			action, ok := actionFor(state)
			assert.Equal(t, want != "", ok)
			assert.Equal(t, want, action)
		})
	}
}

func TestNeedsNetnsOnlyForARunningWorkloadWithItsOwnNetwork(t *testing.T) {
	w := &source.Workload{
		ID:      workloadID,
		Meta:    source.Meta{Networks: map[string]string{"eru-cni": "10.0.0.5"}},
		Running: true,
	}
	assert.True(t, needsNetns(w))

	w.NetnsPID = 42
	assert.False(t, needsNetns(w))

	w.NetnsPID = 0
	w.HostIface = "tap-" + workloadID
	assert.False(t, needsNetns(w))

	w.HostIface = ""
	w.Running = false
	assert.False(t, needsNetns(w))

	assert.False(t, needsNetns(&source.Workload{ID: workloadID, Running: true}))
}
