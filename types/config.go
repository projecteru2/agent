package types

import (
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"
	yaml "gopkg.in/yaml.v2"
)

type DockerConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type MetricsConfig struct {
	Step      int64    `yaml:"step"`
	Transfers []string `yaml:"transfers"`
}

type APIConfig struct {
	Addr string `yaml:"addr"`
}

type LogConfig struct {
	Forwards []string `yaml:"forwards"`
	Stdout   bool     `yaml:"stdout"`
}

type Config struct {
	PidFile             string `yaml:"pid"`
	HealthCheckInterval int    `yaml:"health_check_interval"`
	HealthCheckTimeout  int    `yaml:"health_check_timeout"`
	Core                string `yaml:"core"`
	HostName            string `yaml:"-"`

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
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	config.HostName = hostname

	if c.String("core-endpoint") != "" {
		config.Core = c.String("core-endpoint")
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
}
