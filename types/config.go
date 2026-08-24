package types

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

type DockerConfig struct {
	Endpoint string `yaml:"endpoint" required:"false"`
}

type YavirtConfig struct {
	Endpoint               string   `yaml:"endpoint" required:"false"`
	SkipGuestReportRegexps []string `yaml:"skip_guest_report_regexps" required:"false"`
}

type MetricsConfig struct {
	Step      int64    `yaml:"step" default:"10"`
	Transfers []string `yaml:"transfers"`
}

type APIConfig struct {
	Addr string `yaml:"addr"`
}

type LogConfig struct {
	Forwards []string `yaml:"forwards"`
	Stdout   bool     `yaml:"stdout"`
}

type HealthCheckConfig struct {
	Interval int   `yaml:"interval" default:"60"`
	Timeout  int   `yaml:"timeout" default:"10"`
	CacheTTL int64 `yaml:"cache_ttl" default:"300"`
}

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

// GetHealthCheckStatusTTL returns 0: selfmon lives in eru-core, so core owns the ttl.
func (config *Config) GetHealthCheckStatusTTL() int64 {
	return 0
}

// Prepare overrides the loaded config with the command line flags.
func (config *Config) Prepare(ctx context.Context, c *cli.Command) {
	if c.String("hostname") != "" {
		config.HostName = c.String("hostname")
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			log.WithFunc("Prepare").Fatalf(ctx, err, "Get hostname failed")
		}
		config.HostName = hostname
	}

	if endpoints := c.StringSlice("core-endpoint"); len(endpoints) > 0 {
		config.Core = endpoints
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
		config.HealthCheck.CacheTTL = c.Int64("health-check-cache-ttl")
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

func (config *Config) Print(ctx context.Context) {
	bs, err := yaml.Marshal(config)
	if err != nil {
		log.WithFunc("Print").Fatalf(ctx, err, "print config")
	}

	fmt.Println("---- current config ----")
	scanner := bufio.NewScanner(bytes.NewBuffer(bs))
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	fmt.Println("------------------------")
}
