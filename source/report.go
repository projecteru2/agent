package source

import (
	"sync"

	"github.com/projecteru2/agent/common"
)

// EmitFunc hands one workload's state change to the manager.
type EmitFunc func(ID, action string)

// Reporter drops the state changes that repeat what a source last said about a workload.
type Reporter struct {
	mutex        sync.Mutex
	emit         EmitFunc
	subscription uint64
	last         map[string]string
}

func NewReporter() *Reporter {
	return &Reporter{last: map[string]string{}}
}

func (r *Reporter) Attach(emit EmitFunc) func() {
	r.mutex.Lock()
	r.subscription++
	subscription := r.subscription
	r.emit = emit
	r.mutex.Unlock()

	return func() {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		if r.subscription == subscription {
			r.emit = nil
		}
	}
}

// Report emits action unless it is what this workload was last reported as.
func (r *Reporter) Report(ID, action string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	moved := r.last[ID] != action
	r.last[ID] = action
	if moved && r.emit != nil {
		r.emit(ID, action)
	}
}

// Note records what a listing found: news about a workload already tracked, and a starting point for one that is not.
func (r *Reporter) Note(ID, action string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	last, known := r.last[ID]
	r.last[ID] = action
	if known && last != action && r.emit != nil {
		r.emit(ID, action)
	}
}

func (r *Reporter) Gone(ID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	last, known := r.last[ID]
	if !known {
		return
	}
	if last != common.StatusDie && r.emit != nil {
		r.emit(ID, common.StatusDie)
	}
	delete(r.last, ID)
}

func (r *Reporter) Known(ID string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, ok := r.last[ID]
	return ok
}

// ActionOf turns a runtime's liveness answer into the event action the manager handles.
func ActionOf(running bool) string {
	if running {
		return common.StatusStart
	}
	return common.StatusDie
}
