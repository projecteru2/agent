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
	last  map[string]string
}

func NewReporter() *Reporter {
	return &Reporter{last: map[string]string{}}
}

// Report emits action unless it is what this workload was last reported as.
func (r *Reporter) Report(emit EmitFunc, ID, action string) {
	if r.moved(ID, action) {
		emit(ID, action)
	}
}

// Note records what a listing already told core, so the event stream does not repeat it as news.
func (r *Reporter) Note(ID, action string) {
	r.moved(ID, action)
}

// Forget stops tracking a workload that went away, so it is reported again when it comes back.
func (r *Reporter) Forget(ID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.last, ID)
}

func (r *Reporter) moved(ID, action string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.last[ID] == action {
		return false
	}
	r.last[ID] = action
	return true
}

// ActionOf turns a runtime's liveness answer into the event action the manager handles.
func ActionOf(running bool) string {
	if running {
		return common.StatusStart
	}
	return common.StatusDie
}
