package types

import (
	"testing"
	"time"

	"github.com/jinzhu/configor"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	assert := assert.New(t)

	config := &Config{}
	err := configor.Load(config, "../agent.yaml.sample")
	assert.NoError(err)
	assert.Equal(config.PidFile, "/tmp/agent.pid")
	assert.Equal(config.Core, []string{"127.0.0.1:5001", "127.0.0.1:5002"})
	assert.Equal(config.HostName, "")
	assert.Equal(config.HeartbeatInterval, 120)

	assert.Equal(config.HealthCheck.Interval, 120)
	assert.Equal(config.HealthCheck.Timeout, 10)
	assert.Equal(config.HealthCheck.CacheTTL, 300)
	assert.Equal(config.GetHealthCheckStatusTTL(), int64(0))

	assert.Equal(config.Store, "grpc")
	assert.Equal(config.Runtime, "docker")

	assert.Equal(config.GlobalConnectionTimeout, time.Second*15)

	config.Print()
}
