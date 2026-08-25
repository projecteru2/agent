package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	minimalConfig = `core:
    - 127.0.0.1:5001
`

	samplePath = "../agent.yaml.sample"
)

func TestLoadConfigAppliesDefaults(t *testing.T) {
	config, err := LoadConfig(writeConfig(t, minimalConfig))
	assert.NoError(t, err)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"string default", config.PidFile, "/tmp/agent.pid"},
		{"int default", config.HeartbeatInterval, 60},
		{"bool default", config.CheckOnlyMine, false},
		{"store default", config.Store, "grpc"},
		{"runtime default", config.Runtime, "docker"},
		{"meta dir default", config.MetaDir, "/run/eru/workloads"},
		{"state dir default", config.StateDir, "/var/lib/eru-agent"},
		{"duration default", config.GlobalConnectionTimeout, 5 * time.Second},
		{"metrics step default", config.Metrics.Step, int64(10)},
		{"default in a section the file omits", config.HealthCheck.Interval, 60},
		{"healthcheck timeout default", config.HealthCheck.Timeout, 10},
		{"healthcheck cache ttl default", config.HealthCheck.CacheTTL, int64(300)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestLoadConfigLetsTheFileOverrideDefaults(t *testing.T) {
	config, err := LoadConfig(samplePath)
	assert.NoError(t, err)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"required slice", config.Core, []string{"127.0.0.1:5001", "127.0.0.1:5002"}},
		{"int", config.HeartbeatInterval, 120},
		{"duration", config.GlobalConnectionTimeout, 15 * time.Second},
		{"nested int64", config.Metrics.Step, int64(30)},
		{"nested slice", config.Metrics.Transfers, []string{"127.0.0.1:8125"}},
		{"nested struct field", config.Docker.Endpoint, "unix:///var/run/docker.sock"},
		{"nested slice on a second section", config.Yavirt.SkipGuestReportRegexps, []string{".+002"}},
		{"struct field from another module", config.Auth.Username, "username"},
		{"nested int", config.HealthCheck.Interval, 120},
		{"field the file cannot reach", config.HostName, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestLoadConfigLetsAnExplicitZeroInTheFileWin(t *testing.T) {
	config, err := LoadConfig(writeConfig(t, minimalConfig+"heartbeat_interval: 0\n"))
	assert.NoError(t, err)
	assert.Equal(t, 0, config.HeartbeatInterval)
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing required field", "pid: /tmp/agent.pid\n"},
		{"required field set to null", "core:\n"},
		{"file is not a mapping", "test\n"},
		{"malformed yaml", "pid: \"/tmp/agent.pid\"\n  broken\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, tt.body))
			assert.Error(t, err)
		})
	}
}

func TestLoadConfigNamesTheMissingRequiredField(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "pid: /tmp/agent.pid\n"))
	assert.ErrorContains(t, err, "Core is required, but blank")
}

func TestLoadConfigRejectsAMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "absent.yaml"))
	assert.Error(t, err)
}

func BenchmarkLoadConfig(b *testing.B) {
	for b.Loop() {
		if _, err := LoadConfig(samplePath); err != nil {
			b.Fatalf("load: %v", err)
		}
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
