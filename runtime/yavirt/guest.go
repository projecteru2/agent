package yavirt

import (
	"context"
	"fmt"
	"time"

	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/utils"
)

type Guest struct {
	ID       string
	Networks map[string]string
	Labels   map[string]string
	IPs      []string
	Running  bool
}

func (g *Guest) CheckHealth(ctx context.Context, timeout time.Duration) bool {
	healthCheck := coreutils.DecodeMetaInLabel(ctx, g.Labels).HealthCheck
	if healthCheck == nil {
		return true
	}

	var tcpChecker []string
	var httpChecker []string

	for _, port := range healthCheck.TCPPorts {
		for _, ip := range g.IPs {
			tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", ip, port))
		}
	}
	if healthCheck.HTTPPort != "" {
		for _, ip := range g.IPs {
			httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", ip, healthCheck.HTTPPort, healthCheck.HTTPURL))
		}
	}

	httpOK := utils.CheckHTTP(ctx, g.ID, httpChecker, healthCheck.HTTPCode, timeout)
	tcpOK := utils.CheckTCP(ctx, g.ID, tcpChecker, timeout)
	return httpOK && tcpOK
}
