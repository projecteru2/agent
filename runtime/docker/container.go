package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"
)

// Container docker container
type Container struct {
	coretypes.StatusMeta
	Pid         int
	Name        string
	EntryPoint  string
	Ident       string
	CPUNum      float64
	CPUQuota    int64
	CPUPeriod   int64
	Memory      int64
	Labels      map[string]string
	Env         map[string]string
	HealthCheck *coretypes.HealthCheck
	LocalIP     string `json:"-"`
}

// CheckHealth check container's health status
func (c *Container) CheckHealth(ctx context.Context, timeout time.Duration) bool {
	if c.HealthCheck == nil {
		return true
	}
	var tcpChecker []string
	var httpChecker []string

	for _, port := range c.HealthCheck.TCPPorts {
		tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", c.LocalIP, port))
	}
	if c.HealthCheck.HTTPPort != "" {
		httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", c.LocalIP, c.HealthCheck.HTTPPort, c.HealthCheck.HTTPURL))
	}

	ID := c.ID
	f1 := utils.CheckHTTP(ctx, ID, httpChecker, c.HealthCheck.HTTPCode, timeout)
	f2 := utils.CheckTCP(ctx, ID, tcpChecker, timeout)
	return f1 && f2
}
