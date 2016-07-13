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

var (
	configPath string
	logLevel   string
)

func setupLogLevel(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	return nil
}

func initConfig(configPath string) (types.Config, error) {
	config := types.Config{}

	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}
	return config, nil
}

func serve() error {
	if err := setupLogLevel(logLevel); err != nil {
		log.Fatal(err)
	}

	if configPath == "" {
		log.Fatalf("Config path must be set")
	}

	config, err := initConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	if config.HostName, err = os.Hostname(); err != nil {
		log.Fatal(err)
	}

	if config.Etcd.Prefix == "" {
		config.Etcd.Prefix = common.DEFAULT_ETCD_PREFIX
	}

	log.Debug(config)
	utils.WritePid(config.PidFile)
	defer os.Remove(config.PidFile)

	watcher.InitMonitor()
	go watcher.LogMonitor.Serve()

	agent, err := engine.NewEngine(config)
	if err != nil {
		log.Fatal(err)
	}

	go api.Serve(config.API.Addr)

	return agent.Run()
}

func main() {
	app := cli.NewApp()
	app.Name = "Eru-Agent"
	app.Usage = "Run eru agent"
	app.Version = common.ERU_AGENT_VERSION
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "config",
			Value:       "/etc/eru/agent.yaml",
			Usage:       "config file path for agent, in yaml",
			Destination: &configPath,
			EnvVar:      "ERU_AGENT_CONFIG_PATH",
		},
		cli.StringFlag{
			Name:        "log-level",
			Value:       "INFO",
			Usage:       "set log level",
			Destination: &logLevel,
			EnvVar:      "ERU_AGENT_LOG_LEVEL",
		},
	}
	app.Action = func(c *cli.Context) error {
		return serve()
	}

	app.Run(os.Args)
}
