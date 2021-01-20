package types

import (
	"os"

	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
)

// DockerConfig contain endpoint
type DockerConfig struct {
	Endpoint string `yaml:"endpoint" required:"true"`
}

// MetricsConfig contain metrics config
type MetricsConfig struct {
	Step      int64    `yaml:"step" required:"true" default:"10"`
	Transfers []string `yaml:"transfers"`
}

// APIConfig contain api config
type APIConfig struct {
	Addr string `yaml:"addr"`
}

// LogConfig contain log config
type LogConfig struct {
	Forwards []string `yaml:"forwards"`
	Stdout   bool     `yaml:"stdout"`
}

// HealthCheckConfig contain healthcheck config
type HealthCheckConfig struct {
	Interval  int `yaml:"interval" required:"true" default:"15"`
	StatusTTL int `yaml:"status_ttl"`
	Timeout   int `yaml:"timeout" default:"10"`
	CacheTTL  int `yaml:"cache_ttl" default:"300"`
}

// Config contain all configs
type Config struct {
	PidFile  string `yaml:"pid" required:"true" default:"/tmp/agent.pid"`
	Core     string `yaml:"core" required:"true"`
	HostName string `yaml:"-"`

	Auth              coretypes.AuthConfig `yaml:"auth"`
	Docker            DockerConfig
	Metrics           MetricsConfig
	API               APIConfig
	Log               LogConfig
	HealthCheck       HealthCheckConfig `yaml:"healcheck"`
	HeartbeatInterval int               `yaml:"heartbeat_interval" default:"180"`
}

// PrepareConfig 从cli覆写并做准备
func (config *Config) PrepareConfig(c *cli.Context) {
	if c.String("hostname") != "" {
		config.HostName = c.String("hostname")
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatal(err)
		}
		config.HostName = hostname
	}

	if c.String("core-endpoint") != "" {
		config.Core = c.String("core-endpoint")
	}
	if c.String("core-username") != "" {
		config.Auth.Username = c.String("core-username")
	}
	if c.String("core-password") != "" {
		config.Auth.Password = c.String("core-password")
	}
	if c.String("pidfile") != "" {
		config.PidFile = c.String("pidfile")
	}
	if c.Int("health-check-interval") > 0 {
		config.HealthCheck.Interval = c.Int("health-check-interval")
	}
	config.HealthCheck.StatusTTL = c.Int("health-check-status-ttl") // status ttl can be 0
	if c.Int("health-check-timeout") > 0 {
		config.HealthCheck.Timeout = c.Int("health-check-timeout")
	}
	if c.Int("health-check-cache-ttl") > 0 {
		config.HealthCheck.CacheTTL = c.Int("health-check-cache-ttl")
	}
	if c.String("docker-endpoint") != "" {
		config.Docker.Endpoint = c.String("docker-endpoint")
	}
	if c.Int64("metrics-step") > 0 {
		config.Metrics.Step = c.Int64("metrics-step")
	}
	if len(c.StringSlice("metrics-transfers")) > 0 {
		config.Metrics.Transfers = c.StringSlice("metrics-transfers")
	}
	if c.String("api-addr") != "" {
		config.API.Addr = c.String("api-addr")
	}
	if len(c.StringSlice("log-forwards")) > 0 {
		config.Log.Forwards = c.StringSlice("log-forwards")
	}
	if c.String("log-stdout") != "" {
		config.Log.Stdout = c.String("log-stdout") == "yes"
	}
	// validate
	if config.PidFile == "" {
		log.Fatal("need to set pidfile")
	}
	if config.HealthCheck.Interval == 0 {
		config.HealthCheck.Interval = 15
	}
	if config.HealthCheck.Timeout == 0 {
		config.HealthCheck.Timeout = 10
	}
	if config.HealthCheck.CacheTTL == 0 {
		config.HealthCheck.CacheTTL = 300
	}
}
