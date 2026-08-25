package source

import (
	"sync"

	"github.com/projecteru2/agent/common"
)

// EmitFunc hands one workload's state change to the manager.
type EmitFunc func(ID, action string)

// Reporter drops the state changes that repeat what a source last said about a workload.
type Reporter struct {
	mutex sync.Mutex
	emit  EmitFunc
	last  map[string]string
}

func NewReporter() *Reporter {
	return &Reporter{last: map[string]string{}}
}

func (r *Reporter) Attach(emit EmitFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.emit = emit
}

// Report emits action unless it is what this workload was last reported as.
func (r *Reporter) Report(ID, action string) {
	r.mutex.Lock()
	emit, moved := r.emit, r.last[ID] != action
	r.last[ID] = action
	r.mutex.Unlock()

	if moved && emit != nil {
		emit(ID, action)
	}
}

// Note records what a listing found: news about a workload already tracked, and a starting point for one that is not.
func (r *Reporter) Note(ID, action string) {
	r.mutex.Lock()
	last, known := r.last[ID]
	emit := r.emit
	r.last[ID] = action
	r.mutex.Unlock()

	if known && last != action && emit != nil {
		emit(ID, action)
	}
}

func (r *Reporter) Known(ID string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.last[ID]
	return ok
}

// Forget stops tracking a workload that went away, so it is reported again when it comes back.
func (r *Reporter) Forget(ID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.last, ID)
}

// ActionOf turns a runtime's liveness answer into the event action the manager handles.
func ActionOf(running bool) string {
	if running {
		return common.StatusStart
	}
	return common.StatusDie
}
