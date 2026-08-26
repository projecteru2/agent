package types

import (
	"context"
	"testing"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/projecteru2/agent/common"
)

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

func TestPrepareDefaultsTheRuntimeEndpoints(t *testing.T) {
	config := &Config{
		Core:     []string{"127.0.0.1:5001"},
		Runtimes: RuntimesConfig{Containerd: &ContainerdConfig{}, Cocoon: &CocoonConfig{}},
	}

	require.NoError(t, runPrepare(t.Context(), config, []string{"eru-agent"}))
	require.Equal(t, common.ContainerdSocket, config.Runtimes.Containerd.Socket)
	require.Equal(t, common.ContainerdNamespace, config.Runtimes.Containerd.Namespace)
	require.Equal(t, common.CocoonSocket, config.Runtimes.Cocoon.Socket)
}

func TestPrintRedactsThePassword(t *testing.T) {
	config := &Config{Auth: coretypes.AuthConfig{Username: "eru", Password: "secret"}}

	safe := config.redacted()
	require.Equal(t, "eru", safe.Auth.Username)
	require.Equal(t, "[redacted]", safe.Auth.Password)
	require.Equal(t, "secret", config.Auth.Password)

	bs, err := yaml.Marshal(safe)
	require.NoError(t, err)
	require.NotContains(t, string(bs), "secret")

	config.Print(t.Context())
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
