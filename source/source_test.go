package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogFields(t *testing.T) {
	w := &Workload{
		ID: "with-networks",
		Meta: Meta{
			Podname:  "prod",
			Nodename: "node-1",
			CoreID:   "core-1",
			Networks: map[string]string{"bridge": "10.0.0.5"},
		},
	}

	assert.Equal(t, map[string]string{
		"podname":         "prod",
		"nodename":        "node-1",
		"coreid":          "core-1",
		"networks_bridge": "10.0.0.5",
	}, w.LogFields())
}

func TestLogFieldsOfAWorkloadWithoutNetworks(t *testing.T) {
	w := &Workload{ID: "no-networks"}

	assert.Equal(t, map[string]string{"podname": "", "nodename": "", "coreid": ""}, w.LogFields())
}
