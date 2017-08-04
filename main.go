package main

import (
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/api"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/engine"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
	"gitlab.ricebook.net/platform/agent/watcher"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
)

func setupLogLevel(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	return nil
}

// 从config path指定的文件加载config
// 失败就算了, 反正也要从cli覆写的
func loadConfigFromFile(configPath string) types.Config {
	config := types.Config{}

	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config
	}

	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config
	}
	return config
}

// 从cli覆写
func overrideConfigFromCli(c *cli.Context, config types.Config) types.Config {
	if c.String("pidfile") != "" {
		config.PidFile = c.String("pidfile")
	}
	if c.String("hostname") != "" {
		config.HostName = c.String("hostname")
	}
	if c.String("zone") != "" {
		config.Zone = c.String("zone")
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
	if c.String("etcd-prefix") != "" {
		config.Etcd.Prefix = c.String("etcd-prefix")
	}
	if len(c.StringSlice("etcd")) > 0 {
		config.Etcd.Machines = c.StringSlice("etcd")
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
	if len(c.StringSlice("nic-physical")) > 0 {
		config.NIC.Physical = c.StringSlice("nic-physical")
	}
	return config
}

func initConfig(c *cli.Context) types.Config {
	config := loadConfigFromFile(c.String("config"))
	return overrideConfigFromCli(c, config)
}

func serve(c *cli.Context) error {
	if err := setupLogLevel(c.String("log-level")); err != nil {
		log.Fatal(err)
	}

	config := initConfig(c)
	if hostname, err := os.Hostname(); err != nil {
		log.Fatal(err)
	} else {
		config.HostName = hostname
	}

	if config.Etcd.Prefix == "" {
		config.Etcd.Prefix = common.DEFAULT_ETCD_PREFIX
	}

	if config.PidFile == "" {
		log.Fatal("need to set pidfile")
	}

	log.Debugf("config: %v", config)
	utils.WritePid(config.PidFile)
	defer os.Remove(config.PidFile)

	watcher.InitMonitor()
	go watcher.LogMonitor.Serve()

	agent, err := engine.NewEngine(config)
	if err != nil {
		log.Fatal(err)
	}

	go api.Serve(config.API.Addr)

	if err := agent.Run(); err != nil {
		log.Fatalf("Agent caught error %s", err)
	}
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "Eru-Agent"
	app.Usage = "Run eru agent"
	app.Version = common.ERU_AGENT_VERSION
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Value:  "/etc/eru/agent.yaml",
			Usage:  "config file path for agent, in yaml",
			EnvVar: "ERU_AGENT_CONFIG_PATH",
		},
		cli.StringFlag{
			Name:   "log-level",
			Value:  "INFO",
			Usage:  "set log level",
			EnvVar: "ERU_AGENT_LOG_LEVEL",
		},
		cli.StringFlag{
			Name:   "docker-endpoint",
			Value:  "",
			Usage:  "docker endpoint",
			EnvVar: "ERU_AGENT_DOCKER_ENDPOINT",
		},
		cli.StringFlag{
			Name:   "etcd-prefix",
			Value:  "",
			Usage:  "namespace for agent in etcd storage",
			EnvVar: "ERU_AGENT_ETCD_PREFIX",
		},
		cli.StringSliceFlag{
			Name:   "etcd",
			Value:  &cli.StringSlice{},
			Usage:  "etcd machines, multiple will be ok",
			EnvVar: "ERU_AGENT_ETCD_MACHINES",
		},
		cli.Int64Flag{
			Name:   "metrics-step",
			Value:  0,
			Usage:  "interval for metrics to send",
			EnvVar: "ERU_AGENT_METRICS_STEP",
		},
		cli.StringSliceFlag{
			Name:   "metrics-transfers",
			Value:  &cli.StringSlice{},
			Usage:  "metrics destinations",
			EnvVar: "ERU_AGENT_METRICS_TRANSFERS",
		},
		cli.StringFlag{
			Name:   "api-addr",
			Value:  "",
			Usage:  "agent API serving address",
			EnvVar: "ERU_AGENT_API_ADDR",
		},
		cli.StringSliceFlag{
			Name:   "log-forwards",
			Value:  &cli.StringSlice{},
			Usage:  "log destinations",
			EnvVar: "ERU_AGENT_LOG_FORWARDS",
		},
		cli.StringFlag{
			Name:   "log-stdout",
			Value:  "",
			Usage:  "forward stdout out? yes/no",
			EnvVar: "ERU_AGENT_LOG_STDOUT",
		},
		cli.StringSliceFlag{
			Name:   "nic-physical",
			Value:  &cli.StringSlice{},
			Usage:  "NICs to use",
			EnvVar: "ERU_AGENT_NIC_PHYSICALS",
		},
		cli.StringFlag{
			Name:   "pidfile",
			Value:  "",
			Usage:  "pidfile to save",
			EnvVar: "ERU_AGENT_PIDFILE",
		},
		cli.StringFlag{
			Name:   "hostname",
			Value:  "",
			Usage:  "hostname of agent's host",
			EnvVar: "ERU_AGENT_HOSTNAME",
		},
		cli.StringFlag{
			Name:   "zone",
			Value:  "",
			Usage:  "agent's zone",
			EnvVar: "ERU_AGENT_ZONE",
		},
		cli.IntFlag{
			Name:   "health-check-interval",
			Value:  0,
			Usage:  "interval for agent to check container's health status",
			EnvVar: "ERU_AGENT_HEALTH_CHECK_INTERVAL",
		},
		cli.IntFlag{
			Name:   "health-check-timeout",
			Value:  0,
			Usage:  "timeout for agent to check container's health status",
			EnvVar: "ERU_AGENT_HEALTH_CHECK_TIMEOUT",
		},
	}
	app.Action = func(c *cli.Context) error {
		return serve(c)
	}

	app.Run(os.Args)
}
