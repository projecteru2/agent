package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnitOf(t *testing.T) {
	assert.Equal(t, "eru-"+workloadID+".service", unitOf(workloadID))
}

func TestWorkloadIDFromUnit(t *testing.T) {
	tests := []struct {
		name string
		unit string
		want string
	}{
		{"a workload unit", "eru-" + workloadID + ".service", workloadID},
		{"the agent's own unit", "eru-agent.service", ""},
		{"a core host's unit", "eru-core.service", ""},
		{"an unrelated unit", "sshd.service", ""},
		{"a workload's scope rather than its service", "eru-" + workloadID + ".scope", ""},
		{"an id that is too short", "eru-0f9c1a2b.service", ""},
		{"an id that is too long", "eru-" + workloadID + "ff.service", ""},
		{"an id that is not hex", "eru-zzzc1a2b3d4e5f60718293a4b5c6d7e8.service", ""},
		{"an id in upper case", "eru-0F9C1A2B3D4E5F60718293A4B5C6D7E8.service", ""},
		{"a bare prefix", "eru-.service", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ID, ok := workloadIDFromUnit(tt.unit)
			assert.Equal(t, tt.want != "", ok)
			assert.Equal(t, tt.want, ID)
		})
	}
}
