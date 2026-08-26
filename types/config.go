package types

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"

	"github.com/projecteru2/agent/common"
)

// ContainerdConfig is defaulted in Prepare: the walker cannot default a pointer the file allocates.
type ContainerdConfig struct {
	Socket    string `yaml:"socket"`
	Namespace string `yaml:"namespace"`
}

// SystemdConfig has no keys: a process pod is described by its meta file, not by the runtime.
type SystemdConfig struct{}

// CocoonConfig is defaulted in Prepare like ContainerdConfig; the daemon it points at is optional.
type CocoonConfig struct {
	Socket string `yaml:"socket"`
}

// MocksConfig has no keys: the scripted runtime the test suite runs against.
type MocksConfig struct{}

// RuntimesConfig lists the runtimes this node hosts; the heartbeat needs every one of them alive.
type RuntimesConfig struct {
	Containerd *ContainerdConfig `yaml:"containerd"`
	Systemd    *SystemdConfig    `yaml:"systemd"`
	Cocoon     *CocoonConfig     `yaml:"cocoon"`
	Mocks      *MocksConfig      `yaml:"mocks"`
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

	Store    string `yaml:"store" default:"grpc"`
	MetaDir  string `yaml:"meta_dir" default:"/run/eru/workloads"`
	StateDir string `yaml:"state_dir" default:"/var/lib/eru-agent"`

	Auth     coretypes.AuthConfig `yaml:"auth"`
	Runtimes RuntimesConfig       `yaml:"runtimes"`

	Metrics     MetricsConfig
	API         APIConfig `yaml:"api"`
	Log         LogConfig
	HealthCheck HealthCheckConfig `yaml:"healthcheck"`

	GlobalConnectionTimeout time.Duration `yaml:"global_connection_timeout" default:"5s"`
}

// Prepare overrides the loaded config with the command line flags.
func (config *Config) Prepare(ctx context.Context, c *cli.Command) {
	if c.String("hostname") != "" {
		config.HostName = c.String("hostname")
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			log.WithFunc("types.Prepare").Fatalf(ctx, err, "get hostname")
		}
		config.HostName = hostname
	}

	if endpoints := c.StringSlice("core-endpoint"); len(endpoints) > 0 {
		config.Core = endpoints
	}
	config.Auth.Username = cmp.Or(c.String("core-username"), config.Auth.Username)
	config.Auth.Password = cmp.Or(c.String("core-password"), config.Auth.Password)
	config.PidFile = cmp.Or(c.String("pidfile"), config.PidFile)
	if c.Int("heartbeat-interval") > 0 {
		config.HeartbeatInterval = c.Int("heartbeat-interval")
	}
	if c.Int("health-check-interval") > 0 {
		config.HealthCheck.Interval = c.Int("health-check-interval")
	}
	if c.Int("health-check-timeout") > 0 {
		config.HealthCheck.Timeout = c.Int("health-check-timeout")
	}
	if c.Int64("health-check-cache-ttl") > 0 {
		config.HealthCheck.CacheTTL = c.Int64("health-check-cache-ttl")
	}
	if c.Int64("metrics-step") > 0 {
		config.Metrics.Step = c.Int64("metrics-step")
	}
	if len(c.StringSlice("metrics-transfers")) > 0 {
		config.Metrics.Transfers = c.StringSlice("metrics-transfers")
	}
	config.API.Addr = cmp.Or(c.String("api-addr"), config.API.Addr)
	if len(c.StringSlice("log-forwards")) > 0 {
		config.Log.Forwards = c.StringSlice("log-forwards")
	}
	if c.String("log-stdout") != "" {
		config.Log.Stdout = c.String("log-stdout") == "yes"
	}
	if c.Bool("check-only-mine") {
		config.CheckOnlyMine = true
	}
	config.Store = cmp.Or(c.String("store"), config.Store)
	if containerd := config.Runtimes.Containerd; containerd != nil {
		containerd.Socket = cmp.Or(containerd.Socket, common.ContainerdSocket)
		containerd.Namespace = cmp.Or(containerd.Namespace, common.ContainerdNamespace)
	}
	if cocoon := config.Runtimes.Cocoon; cocoon != nil {
		cocoon.Socket = cmp.Or(cocoon.Socket, common.CocoonSocket)
	}
}

func (config *Config) Print(ctx context.Context) {
	bs, err := yaml.Marshal(config.redacted())
	if err != nil {
		log.WithFunc("types.Print").Fatalf(ctx, err, "print config")
	}

	fmt.Println("---- current config ----")
	scanner := bufio.NewScanner(bytes.NewBuffer(bs))
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	fmt.Println("------------------------")
}

func (config *Config) redacted() *Config {
	safe := *config
	if safe.Auth.Password != "" {
		safe.Auth.Password = "[redacted]"
	}
	return &safe
}
