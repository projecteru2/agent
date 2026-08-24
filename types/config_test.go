package types

import (
	"context"
	"testing"
	"time"

	"github.com/jinzhu/configor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
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
	assert.Equal(config.HealthCheck.CacheTTL, int64(300))
	assert.Equal(config.GetHealthCheckStatusTTL(), int64(0))

	assert.Equal(config.Store, "grpc")
	assert.Equal(config.Runtime, "docker")

	assert.Equal(config.GlobalConnectionTimeout, time.Second*15)

	config.Print(t.Context())
}

func TestPrepareCoreEndpoints(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "no flag keeps the configured endpoints",
			args: []string{"eru-agent"},
			want: []string{"127.0.0.1:5001"},
		},
		{
			name: "one flag replaces the configured endpoints",
			args: []string{"eru-agent", "--core-endpoint", "10.0.0.1:5001"},
			want: []string{"10.0.0.1:5001"},
		},
		{
			name: "repeated flags collect every endpoint",
			args: []string{"eru-agent", "--core-endpoint", "10.0.0.1:5001", "--core-endpoint", "10.0.0.2:5001"},
			want: []string{"10.0.0.1:5001", "10.0.0.2:5001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Core: []string{"127.0.0.1:5001"}}
			require.NoError(t, runPrepare(t.Context(), config, tt.args))
			require.Equal(t, tt.want, config.Core)
		})
	}
}

func runPrepare(ctx context.Context, config *Config, args []string) error {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "core-endpoint"},
			&cli.StringFlag{Name: "hostname", Value: "fake"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			config.Prepare(ctx, cmd)
			return nil
		},
	}
	return cmd.Run(ctx, args)
}
