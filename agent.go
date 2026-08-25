package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jinzhu/configor"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	zerolog "github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/projecteru2/agent/api"
	"github.com/projecteru2/agent/manager/node"
	"github.com/projecteru2/agent/manager/workload"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/agent/version"
)

func initConfig(ctx context.Context, cmd *cli.Command) (*types.Config, error) {
	config := &types.Config{}

	if err := configor.Load(config, cmd.String("config")); err != nil {
		return nil, err
	}

	config.Prepare(ctx, cmd)
	config.Print(ctx)
	return config, nil
}

func serve(ctx context.Context, cmd *cli.Command) error {
	if err := log.SetupLog(ctx, &coretypes.ServerLogConfig{Level: cmd.String("log-level")}, ""); err != nil {
		zerolog.Fatal().Err(err).Send()
	}

	logger := log.WithFunc("main")

	config, err := initConfig(ctx, cmd)
	if err != nil {
		logger.Fatalf(ctx, err, "load config")
	}
	utils.WritePid(ctx, config.PidFile)
	defer func() { _ = os.Remove(config.PidFile) }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1)
	errChan := make(chan error, 2)
	defer close(errChan)

	var wg sync.WaitGroup

	workloadsManager, err := workload.NewManager(ctx, config)
	if err != nil {
		return err
	}
	wg.Go(func() {
		if runErr := workloadsManager.Run(ctx); runErr != nil {
			logger.Error(ctx, runErr, "[agent] workload manager failed")
			errChan <- runErr
		}
	})

	nodeManager, err := node.NewManager(ctx, config)
	if err != nil {
		return err
	}
	wg.Go(func() {
		if runErr := nodeManager.Run(ctx); runErr != nil {
			logger.Error(ctx, runErr, "[agent] node manager failed")
			errChan <- runErr
		}
	})

	apiHandler := api.NewHandler(config, workloadsManager)
	go apiHandler.Serve(ctx)

	go func() {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "[agent] Agent exiting")
		case runErr := <-errChan:
			logger.Error(ctx, runErr, "[agent] Got error, exiting")
			cancel()
		case sig := <-signalChan:
			logger.Infof(ctx, "[agent] Agent caught system signal %v", sig)
			if sig != syscall.SIGUSR1 {
				if exitErr := nodeManager.Exit(ctx); exitErr != nil {
					logger.Error(ctx, exitErr, "[agent] node manager exits with err")
				}
			}
			cancel()
		}
	}()

	wg.Wait()
	return nil
}

func main() {
	cli.VersionPrinter = func(_ *cli.Command) {
		fmt.Print(version.String())
	}

	app := &cli.Command{
		Name:    version.NAME,
		Usage:   "Run eru agent",
		Version: version.VERSION,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Value:   "/etc/eru/agent.yaml",
				Usage:   "config file path for agent, in yaml",
				Sources: cli.EnvVars("ERU_AGENT_CONFIG_PATH"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "INFO",
				Usage:   "set log level",
				Sources: cli.EnvVars("ERU_AGENT_LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "store",
				Usage:   "store type",
				Sources: cli.EnvVars("ERU_AGENT_STORE"),
			},
			&cli.StringSliceFlag{
				Name:    "core-endpoint",
				Usage:   "core endpoints",
				Sources: cli.EnvVars("ERU_AGENT_CORE_ENDPOINT"),
			},
			&cli.StringFlag{
				Name:    "core-username",
				Usage:   "core username",
				Sources: cli.EnvVars("ERU_AGENT_CORE_USERNAME"),
			},
			&cli.StringFlag{
				Name:    "core-password",
				Usage:   "core password",
				Sources: cli.EnvVars("ERU_AGENT_CORE_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "runtime",
				Usage:   "runtime type",
				Sources: cli.EnvVars("ERU_AGENT_RUNTIME"),
			},
			&cli.StringFlag{
				Name:    "docker-endpoint",
				Usage:   "docker endpoint",
				Sources: cli.EnvVars("ERU_AGENT_DOCKER_ENDPOINT"),
			},
			&cli.Int64Flag{
				Name:    "metrics-step",
				Usage:   "interval for metrics to send",
				Sources: cli.EnvVars("ERU_AGENT_METRICS_STEP"),
			},
			&cli.StringSliceFlag{
				Name:    "metrics-transfers",
				Usage:   "metrics destinations",
				Sources: cli.EnvVars("ERU_AGENT_METRICS_TRANSFERS"),
			},
			&cli.StringFlag{
				Name:    "api-addr",
				Usage:   "agent api serving address",
				Sources: cli.EnvVars("ERU_AGENT_API_ADDR"),
			},
			&cli.StringSliceFlag{
				Name:    "log-forwards",
				Usage:   "log destinations",
				Sources: cli.EnvVars("ERU_AGENT_LOG_FORWARDS"),
			},
			&cli.StringFlag{
				Name:    "log-stdout",
				Usage:   "forward stdout out? yes/no",
				Sources: cli.EnvVars("ERU_AGENT_LOG_STDOUT"),
			},
			&cli.StringFlag{
				Name:    "pidfile",
				Usage:   "pidfile to save",
				Sources: cli.EnvVars("ERU_AGENT_PIDFILE"),
			},
			&cli.IntFlag{
				Name:    "health-check-interval",
				Usage:   "interval for agent to check container's health status",
				Sources: cli.EnvVars("ERU_AGENT_HEALTH_CHECK_INTERVAL"),
			},
			&cli.IntFlag{
				Name:    "health-check-timeout",
				Usage:   "timeout for agent to check container's health status",
				Sources: cli.EnvVars("ERU_AGENT_HEALTH_CHECK_TIMEOUT"),
			},
			&cli.IntFlag{
				Name:    "health-check-cache-ttl",
				Usage:   "ttl for container's health status in local memory",
				Sources: cli.EnvVars("ERU_AGENT_HEALTH_CHECK_CACHE_TTL"),
			},
			&cli.IntFlag{
				Name:    "heartbeat-interval",
				Usage:   "interval for agent to send heartbeat to core",
				Sources: cli.EnvVars("ERU_AGENT_HEARTBEAT_INTERVAL"),
			},
			&cli.StringFlag{
				Name:    "hostname",
				Usage:   "change hostname",
				Sources: cli.EnvVars("ERU_HOSTNAME"),
			},
			&cli.BoolFlag{
				Name:  "check-only-mine",
				Usage: "will only check containers belong to this node if set",
			},
		},
		Action: serve,
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		zerolog.Fatal().Err(err).Send()
	}
}
