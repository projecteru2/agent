package source

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
)

const (
	oneWorkload     = "0f9c1a2b3d4e5f60718293a4b5c6d7e8"
	anotherWorkload = "1a2b3c4d5e6f708192a3b4c5d6e7f809"
)

func TestReportOnlyEmitsAnActionThatMoved(t *testing.T) {
	r, emitted := attached()

	for range 5 {
		r.Report(oneWorkload, common.StatusStart)
	}
	for range 5 {
		r.Report(oneWorkload, common.StatusDie)
	}
	r.Report(oneWorkload, common.StatusStart)

	assert.Equal(t, []string{common.StatusStart, common.StatusDie, common.StatusStart}, *emitted)
}

func TestReportTracksEachWorkloadOnItsOwn(t *testing.T) {
	r := NewReporter()
	var ids []string
	r.Attach(func(ID, _ string) { ids = append(ids, ID) })

	r.Report(oneWorkload, common.StatusStart)
	r.Report(anotherWorkload, common.StatusStart)
	r.Report(oneWorkload, common.StatusStart)

	assert.Equal(t, []string{oneWorkload, anotherWorkload}, ids)
}

func TestNoteSeedsAWorkloadItHasNotSeen(t *testing.T) {
	r, emitted := attached()

	r.Note(oneWorkload, common.StatusStart)
	assert.Empty(t, *emitted)

	r.Report(oneWorkload, common.StatusStart)
	assert.Empty(t, *emitted)

	r.Report(oneWorkload, common.StatusDie)
	assert.Equal(t, []string{common.StatusDie}, *emitted)
}

func TestNoteEmitsATransitionOfAWorkloadItAlreadyTracks(t *testing.T) {
	r, emitted := attached()

	r.Report(oneWorkload, common.StatusStart)
	r.Note(oneWorkload, common.StatusDie)
	r.Note(oneWorkload, common.StatusDie)

	assert.Equal(t, []string{common.StatusStart, common.StatusDie}, *emitted)
}

func TestReportWithoutASubscriptionOnlyRemembers(t *testing.T) {
	r := NewReporter()

	r.Report(oneWorkload, common.StatusStart)
	assert.True(t, r.Known(oneWorkload))
}

func TestForgetLetsAWorkloadBeReportedAgain(t *testing.T) {
	r, emitted := attached()

	r.Report(oneWorkload, common.StatusStart)
	r.Report(oneWorkload, common.StatusStart)
	r.Forget(oneWorkload)
	assert.False(t, r.Known(oneWorkload))

	r.Report(oneWorkload, common.StatusStart)
	assert.Equal(t, []string{common.StatusStart, common.StatusStart}, *emitted)
}

func TestOldSubscriptionCannotDetachItsReplacement(t *testing.T) {
	r := NewReporter()
	oldDetach := r.Attach(func(string, string) {})
	var emitted []string
	r.Attach(func(_, action string) { emitted = append(emitted, action) })

	oldDetach()
	r.Report(oneWorkload, common.StatusStart)

	assert.Equal(t, []string{common.StatusStart}, emitted)
}

func TestActionOf(t *testing.T) {
	assert.Equal(t, common.StatusStart, ActionOf(true))
	assert.Equal(t, common.StatusDie, ActionOf(false))
}

func attached() (*Reporter, *[]string) {
	r := NewReporter()
	actions := &[]string{}
	r.Attach(func(_, action string) { *actions = append(*actions, action) })
	return r, actions
}
