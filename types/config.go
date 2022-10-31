package types

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"time"

	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/core/log"
	cli "github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// DockerConfig contain docker endpoint
type DockerConfig struct {
	Endpoint string `yaml:"endpoint" required:"false"`
}

// YavirtConfig contain yavirt endpoint
type YavirtConfig struct {
	Endpoint               string   `yaml:"endpoint" required:"false"`
	SkipGuestReportRegexps []string `yaml:"skip_guest_report_regexps" required:"false"`
}

// MetricsConfig contain metrics config
type MetricsConfig struct {
	Step      int64    `yaml:"step" default:"10"`
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
	Interval int `yaml:"interval" default:"60"`
	Timeout  int `yaml:"timeout" default:"10"`
	CacheTTL int `yaml:"cache_ttl" default:"300"`
}

// Config contain all configs
type Config struct {
	PidFile           string   `yaml:"pid" default:"/tmp/agent.pid"`
	Core              []string `yaml:"core" required:"true"`
	HostName          string   `yaml:"-"`
	HeartbeatInterval int      `yaml:"heartbeat_interval" default:"60"`

	CheckOnlyMine bool `yaml:"check_only_mine" default:"false"`

	Store   string `yaml:"store" default:"grpc"`
	Runtime string `yaml:"runtime" default:"docker"`

	Auth   coretypes.AuthConfig `yaml:"auth"`
	Docker DockerConfig
	Yavirt YavirtConfig

	Metrics     MetricsConfig
	API         APIConfig `yaml:"api"`
	Log         LogConfig
	HealthCheck HealthCheckConfig `yaml:"healthcheck"`

	GlobalConnectionTimeout time.Duration `yaml:"global_connection_timeout" default:"5s"`
}

// GetHealthCheckStatusTTL returns the TTL for health check status.
// Because selfmon is integrated in eru-core, so there returns 0.
func (config *Config) GetHealthCheckStatusTTL() int64 {
	return 0
}

// Prepare 从 cli 覆写并做准备
func (config *Config) Prepare(c *cli.Context) {
	if c.String("hostname") != "" {
		config.HostName = c.String("hostname")
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf(c.Context, err, "Get hostname failed %v", err)
		}
		config.HostName = hostname
	}

	if c.String("core-endpoint") != "" {
		config.Core = c.StringSlice("core-endpoint")
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
	if c.Int("heartbeat-interval") > 0 {
		config.HeartbeatInterval = c.Int("heartbeat-interval")
	}
	if c.Int("health-check-interval") > 0 {
		config.HealthCheck.Interval = c.Int("health-check-interval")
	}
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
	if c.Bool("check-only-mine") {
		config.CheckOnlyMine = true
	}
	if c.String("runtime") != "" {
		config.Runtime = c.String("runtime")
	}
	if c.String("store") != "" {
		config.Store = c.String("store")
	}
	// validate
	if config.PidFile == "" {
		config.PidFile = "./agent.pid"
	}
	if config.HealthCheck.Interval == 0 {
		config.HealthCheck.Interval = 60
	}
	if config.HealthCheck.Timeout == 0 {
		config.HealthCheck.Timeout = 10
	}
	if config.HealthCheck.CacheTTL == 0 {
		config.HealthCheck.CacheTTL = 300
	}
}

// Print config
func (config *Config) Print() {
	bs, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf(nil, err, "[config] print config failed %v", err) //nolint
	}

	fmt.Println("---- current config ----")
	scanner := bufio.NewScanner(bytes.NewBuffer(bs))
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	fmt.Println("------------------------")
}
