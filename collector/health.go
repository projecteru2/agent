package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/utils"
)

// Probe reports whether the workload answers the health check core declared for it.
func Probe(ctx context.Context, w *source.Workload, timeout time.Duration) bool {
	check := w.Meta.HealthCheck
	if check == nil {
		return true
	}

	var tcpChecker []string
	var httpChecker []string

	for _, port := range check.TCPPorts {
		tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", w.LocalIP, port))
	}
	if check.HTTPPort != "" {
		httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", w.LocalIP, check.HTTPPort, check.HTTPURL))
	}

	httpOK := utils.CheckHTTP(ctx, w.ID, httpChecker, check.HTTPCode, timeout)
	tcpOK := utils.CheckTCP(ctx, w.ID, tcpChecker, timeout)
	return httpOK && tcpOK
}
