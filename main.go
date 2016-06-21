package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
	"gopkg.in/urfave/cli.v1"
	"gopkg.in/yaml.v2"
)

var (
	configPath string
	logLevel   string
)

func waitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
	log.Info("Terminating...")
}

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

func serve() {
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

	waitSignal()
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
		serve()
		return nil
	}

	app.Run(os.Args)
}
