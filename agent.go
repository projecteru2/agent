package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/projecteru2/agent/api"
	"github.com/projecteru2/agent/manager/node"
	"github.com/projecteru2/agent/manager/workload"
	"github.com/projecteru2/agent/selfmon"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/agent/version"

	"github.com/jinzhu/configor"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	_ "go.uber.org/automaxprocs"
)

func setupLogLevel(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)
	log.SetOutput(os.Stdout)
	return nil
}

func initConfig(c *cli.Context) *types.Config {
	config := &types.Config{}

	if err := configor.Load(config, c.String("config")); err != nil {
		log.Fatalf("[main] load config failed %v", err)
	}

	config.Prepare(c)
	config.Print()
	return config
}

func serve(c *cli.Context) error {
	rand.Seed(time.Now().UnixNano())

	if err := setupLogLevel(c.String("log-level")); err != nil {
		log.Fatal(err)
	}

	config := initConfig(c)
	utils.WritePid(config.PidFile)
	defer os.Remove(config.PidFile)

	if c.Bool("selfmon") {
		mon, err := selfmon.New(c.Context, config)
		if err != nil {
			return err
		}
		return mon.Run(c.Context)
	}

	ctx, cancel := signal.NotifyContext(c.Context, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	errChan := make(chan error, 2)
	defer close(errChan)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	workloadManager, err := workload.NewManager(ctx, config)
	if err != nil {
		return err
	}
	go func() {
		defer wg.Done()
		if err := workloadManager.Run(ctx); err != nil {
			log.Errorf("[agent] workload manager err: %v, exiting", err)
			errChan <- err
		}
	}()

	nodeManager, err := node.NewManager(ctx, config)
	if err != nil {
		return err
	}
	go func() {
		defer wg.Done()
		if err := nodeManager.Run(ctx); err != nil {
			log.Errorf("[agent] node manager err: %v, exiting", err)
			errChan <- err
		}
	}()

	apiHandler := api.NewHandler(config, workloadManager)
	go apiHandler.Serve()

	go func() {
		select {
		case <-ctx.Done():
			log.Info("[agent] Agent caught system signal, exiting")
		case <-errChan:
			log.Info("[agent] got err, exiting")
			cancel()
		}
	}()

	wg.Wait()
	return nil
}

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Print(version.String())
	}

	app := &cli.App{
		Name:    version.NAME,
		Usage:   "Run eru agent",
		Version: version.VERSION,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Value:   "/etc/eru/agent.yaml",
				Usage:   "config file path for agent, in yaml",
				EnvVars: []string{"ERU_AGENT_CONFIG_PATH"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "INFO",
				Usage:   "set log level",
				EnvVars: []string{"ERU_AGENT_LOG_LEVEL"},
			},
			&cli.StringFlag{
				Name:    "store",
				Value:   "",
				Usage:   "store type",
				EnvVars: []string{"ERU_AGENT_STORE"},
			},
			&cli.StringFlag{
				Name:    "core-endpoint",
				Value:   "",
				Usage:   "core endpoint",
				EnvVars: []string{"ERU_AGENT_CORE_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:    "core-username",
				Value:   "",
				Usage:   "core username",
				EnvVars: []string{"ERU_AGENT_CORE_USERNAME"},
			},
			&cli.StringFlag{
				Name:    "core-password",
				Value:   "",
				Usage:   "core password",
				EnvVars: []string{"ERU_AGENT_CORE_PASSWORD"},
			},
			&cli.StringFlag{
				Name:    "runtime",
				Value:   "",
				Usage:   "runtime type",
				EnvVars: []string{"ERU_AGENT_RUNTIME"},
			},
			&cli.StringFlag{
				Name:    "docker-endpoint",
				Value:   "",
				Usage:   "docker endpoint",
				EnvVars: []string{"ERU_AGENT_DOCKER_ENDPOINT"},
			},
			&cli.Int64Flag{
				Name:    "metrics-step",
				Value:   0,
				Usage:   "interval for metrics to send",
				EnvVars: []string{"ERU_AGENT_METRICS_STEP"},
			},
			&cli.StringSliceFlag{
				Name:    "metrics-transfers",
				Value:   &cli.StringSlice{},
				Usage:   "metrics destinations",
				EnvVars: []string{"ERU_AGENT_METRICS_TRANSFERS"},
			},
			&cli.StringFlag{
				Name:    "api-addr",
				Value:   "",
				Usage:   "agent api serving address",
				EnvVars: []string{"ERU_AGENT_API_ADDR"},
			},
			&cli.StringSliceFlag{
				Name:    "log-forwards",
				Value:   &cli.StringSlice{},
				Usage:   "log destinations",
				EnvVars: []string{"ERU_AGENT_LOG_FORWARDS"},
			},
			&cli.StringFlag{
				Name:    "log-stdout",
				Value:   "",
				Usage:   "forward stdout out? yes/no",
				EnvVars: []string{"ERU_AGENT_LOG_STDOUT"},
			},
			&cli.StringFlag{
				Name:    "pidfile",
				Value:   "",
				Usage:   "pidfile to save",
				EnvVars: []string{"ERU_AGENT_PIDFILE"},
			},
			&cli.IntFlag{
				Name:    "health-check-interval",
				Usage:   "interval for agent to check container's health status",
				EnvVars: []string{"ERU_AGENT_HEALTH_CHECK_INTERVAL"},
			},
			&cli.IntFlag{
				Name:    "health-check-timeout",
				Usage:   "timeout for agent to check container's health status",
				EnvVars: []string{"ERU_AGENT_HEALTH_CHECK_TIMEOUT"},
			},
			&cli.IntFlag{
				Name:    "health-check-cache-ttl",
				Usage:   "ttl for container's health status in local memory",
				EnvVars: []string{"ERU_AGENT_HEALTH_CHECK_CACHE_TTL"},
			},
			&cli.IntFlag{
				Name:    "heartbeat-interval",
				Usage:   "interval for agent to send heartbeat to core",
				EnvVars: []string{"ERU_AGENT_HEARTBEAT_INTERVAL"},
			},
			&cli.StringFlag{
				Name:    "hostname",
				Value:   "",
				Usage:   "change hostname",
				EnvVars: []string{"ERU_HOSTNAME"},
			},
			&cli.BoolFlag{
				Name:  "selfmon",
				Value: false,
				Usage: "run this agent as a selfmon daemon",
			},
			&cli.StringFlag{
				Name:    "kv",
				Value:   "",
				Usage:   "kv type",
				EnvVars: []string{"ERU_AGENT_KV"},
			},
			&cli.BoolFlag{
				Name:  "check-only-mine",
				Value: false,
				Usage: "will only check containers belong to this node if set",
			},
		},
		Action: serve,
	}
	if err := app.Run(os.Args); err != nil {
		log.Errorf("Error running agent: %v", err)
	}
}
