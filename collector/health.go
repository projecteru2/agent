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
	// without an address of its own there is nothing to dial, and the node's own ports are not the workload's
	if w.LocalIP == "" {
		return false
	}

	var tcpChecker []string
	for _, port := range check.TCPPorts {
		tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", w.LocalIP, port))
	}
	httpURL := ""
	if check.HTTPPort != "" {
		httpURL = fmt.Sprintf("http://%s:%s%s", w.LocalIP, check.HTTPPort, check.HTTPURL)
	}

	httpOK := utils.CheckHTTP(ctx, w.ID, httpURL, check.HTTPCode, timeout)
	tcpOK := utils.CheckTCP(ctx, w.ID, tcpChecker, timeout)
	return httpOK && tcpOK
}
