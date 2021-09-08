package node

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/assert"
)

func newMockNodeManager(t *testing.T) *Manager {
	config := &types.Config{
		HostName:          "fake",
		HeartbeatInterval: 2,
		CheckOnlyMine:     false,
		Store:             common.MocksStore,
		Runtime:           common.MocksRuntime,
		Log: types.LogConfig{
			Stdout: true,
		},
		HealthCheck: types.HealthCheckConfig{
			Interval:      10,
			Timeout:       5,
			CacheTTL:      300,
			EnableSelfmon: true,
		},
		GlobalConnectionTimeout: 5 * time.Second,
	}

	m, err := NewManager(context.Background(), config)
	assert.Nil(t, err)
	return m
}
