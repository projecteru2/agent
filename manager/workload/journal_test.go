package workload

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/source"
)

func TestLogTargetResolvesByWorkloadIDThenByUnit(t *testing.T) {
	m, target := managerWithTarget()

	assert.Same(t, target, m.logTarget(&collector.Entry{WorkloadID: "abc123", Unit: "containerd.service"}))
	assert.Same(t, target, m.logTarget(&collector.Entry{Unit: "eru-abc123.service"}))
	assert.Nil(t, m.logTarget(&collector.Entry{Unit: "sshd.service"}))
	assert.Nil(t, m.logTarget(&collector.Entry{}))
}

func TestStopForwardingDropsEveryKeyOfTheWorkload(t *testing.T) {
	m, _ := managerWithTarget()

	m.stopForwarding("abc123")
	assert.Empty(t, m.logTargets)
}

func TestLogKeysOfAWorkloadWithoutAUnit(t *testing.T) {
	assert.Equal(t, []string{"abc123"}, logKeys(&source.Workload{ID: "abc123"}))
}

func managerWithTarget() (*Manager, *logTarget) {
	w := &source.Workload{ID: "abc123", Log: source.Log{JournalUnit: "eru-abc123.service"}}
	target := &logTarget{workload: w, cancel: func() {}}

	m := &Manager{logTargets: map[string]*logTarget{}}
	for _, key := range logKeys(w) {
		m.logTargets[key] = target
	}
	return m, target
}
