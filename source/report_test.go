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
	r := NewReporter()
	emit, emitted := recorder()

	for range 5 {
		r.Report(emit, oneWorkload, common.StatusStart)
	}
	for range 5 {
		r.Report(emit, oneWorkload, common.StatusDie)
	}
	r.Report(emit, oneWorkload, common.StatusStart)

	assert.Equal(t, []string{common.StatusStart, common.StatusDie, common.StatusStart}, *emitted)
}

func TestReportTracksEachWorkloadOnItsOwn(t *testing.T) {
	r := NewReporter()
	var ids []string
	emit := EmitFunc(func(ID, _ string) { ids = append(ids, ID) })

	r.Report(emit, oneWorkload, common.StatusStart)
	r.Report(emit, anotherWorkload, common.StatusStart)
	r.Report(emit, oneWorkload, common.StatusStart)

	assert.Equal(t, []string{oneWorkload, anotherWorkload}, ids)
}

func TestNoteKeepsAListedStateFromBeingNews(t *testing.T) {
	r := NewReporter()
	emit, emitted := recorder()

	r.Note(oneWorkload, common.StatusStart)
	r.Report(emit, oneWorkload, common.StatusStart)
	r.Report(emit, oneWorkload, common.StatusDie)

	assert.Equal(t, []string{common.StatusDie}, *emitted)
}

func TestActionOf(t *testing.T) {
	assert.Equal(t, common.StatusStart, ActionOf(true))
	assert.Equal(t, common.StatusDie, ActionOf(false))
}

func TestForgetLetsAWorkloadBeReportedAgain(t *testing.T) {
	r := NewReporter()
	emit, emitted := recorder()

	r.Report(emit, oneWorkload, common.StatusStart)
	r.Report(emit, oneWorkload, common.StatusStart)
	r.Forget(oneWorkload)
	r.Report(emit, oneWorkload, common.StatusStart)

	assert.Equal(t, []string{common.StatusStart, common.StatusStart}, *emitted)
}

func recorder() (EmitFunc, *[]string) {
	actions := &[]string{}
	return func(_, action string) { *actions = append(*actions, action) }, actions
}
