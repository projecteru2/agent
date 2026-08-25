package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
)

func TestEmitChangeOnlyReportsAnActionThatMoved(t *testing.T) {
	s := &Systemd{reported: map[string]string{}}
	var emitted []string
	emit := emitFunc(func(_, action string) { emitted = append(emitted, action) })

	for range 5 {
		s.emitChange(emit, cniWorkload, common.StatusStart)
	}
	for range 5 {
		s.emitChange(emit, cniWorkload, common.StatusDie)
	}
	s.emitChange(emit, cniWorkload, common.StatusStart)

	assert.Equal(t, []string{common.StatusStart, common.StatusDie, common.StatusStart}, emitted)
}

func TestEmitChangeTreatsFailedAndInactiveAsOneDeath(t *testing.T) {
	s := &Systemd{reported: map[string]string{}}
	var emitted []string
	emit := emitFunc(func(_, action string) { emitted = append(emitted, action) })

	s.emitChange(emit, cniWorkload, common.StatusStart)
	for _, state := range []string{stateFailed, stateInactive} {
		action, ok := actionFor(state)
		assert.True(t, ok)
		s.emitChange(emit, cniWorkload, action)
	}

	assert.Equal(t, []string{common.StatusStart, common.StatusDie}, emitted)
}

func TestEmitChangeReportsEachUnitOnItsOwn(t *testing.T) {
	s := &Systemd{reported: map[string]string{}}
	var emitted []string
	emit := emitFunc(func(ID, _ string) { emitted = append(emitted, ID) })

	s.emitChange(emit, cniWorkload, common.StatusStart)
	s.emitChange(emit, hostNetWorkload, common.StatusStart)
	s.emitChange(emit, cniWorkload, common.StatusStart)

	assert.Equal(t, []string{cniWorkload, hostNetWorkload}, emitted)
}

func TestForgetLetsARemovedUnitBeReportedAgain(t *testing.T) {
	s := &Systemd{reported: map[string]string{}}

	assert.True(t, s.report(unitOf(cniWorkload), common.StatusStart))
	assert.False(t, s.report(unitOf(cniWorkload), common.StatusStart))

	s.forget(unitOf(cniWorkload))
	assert.True(t, s.report(unitOf(cniWorkload), common.StatusStart))
}

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
