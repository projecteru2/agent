package types

import (
	"io/ioutil"
	"os"

	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/urfave/cli.v2"
	yaml "gopkg.in/yaml.v2"
)

// DockerConfig contain endpoint
type DockerConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// MetricsConfig contain metrics config
type MetricsConfig struct {
	Step      int64    `yaml:"step"`
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

// Config contain all configs
type Config struct {
	PidFile             string               `yaml:"pid"`
	HealthCheckInterval int                  `yaml:"health_check_interval"`
	HealthCheckTimeout  int                  `yaml:"health_check_timeout"`
	HealthCheckCacheTTL int                  `yaml:"health_check_cache_ttl"`
	Core                string               `yaml:"core"`
	Auth                coretypes.AuthConfig `yaml:"auth"`
	HostName            string               `yaml:"-"`

	Docker  DockerConfig
	Metrics MetricsConfig
	API     APIConfig
	Log     LogConfig
}

//LoadConfigFromFile 从config path指定的文件加载config
//失败就算了, 反正也要从cli覆写的
func (config *Config) LoadConfigFromFile(configPath string) error {
	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(bytes, config)
}

//PrepareConfig 从cli覆写并做准备
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
		config.HealthCheckInterval = c.Int("health-check-interval")
	}
	if c.Int("health-check-timeout") > 0 {
		config.HealthCheckTimeout = c.Int("health-check-timeout")
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
	//validate
	if config.PidFile == "" {
		log.Fatal("need to set pidfile")
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 3
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10
	}
	if config.HealthCheckCacheTTL == 0 {
		config.HealthCheckCacheTTL = 60
	}
}
