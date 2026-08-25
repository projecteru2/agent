package yavirt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/utils"
)

const LabelMeta = "ERU_META"

type HealthCheck struct {
	TCPPorts []string
	HTTPPort string
	HTTPURL  string
	HTTPCode int
}

type healthCheckMeta struct {
	Publish     []string
	HealthCheck *HealthCheck
}

type Guest struct {
	ID            string
	Status        string
	TransitStatus string
	CreateTime    int64
	TransitTime   int64
	UpdateTime    int64
	CPU           int
	Mem           int64
	Storage       int64
	ImageID       int64
	ImageName     string
	ImageUser     string
	Networks      map[string]string
	Labels        map[string]string
	IPs           []string
	Hostname      string
	Running       bool
	HealthCheck   *HealthCheck

	once sync.Once
}

func (g *Guest) CheckHealth(ctx context.Context, timeout time.Duration) bool {
	g.once.Do(func() {
		if meta, ok := g.Labels[LabelMeta]; ok {
			hcm := &healthCheckMeta{}
			err := json.Unmarshal([]byte(meta), hcm)
			if err != nil {
				log.WithFunc("yavirt.CheckHealth").Errorf(ctx, err, "invalid json format, guest %v, meta %v", g.ID, meta)
				return
			}
			g.HealthCheck = hcm.HealthCheck
		}
	})

	if g.HealthCheck == nil {
		return true
	}

	var tcpChecker []string
	var httpChecker []string

	healthCheck := g.HealthCheck

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
