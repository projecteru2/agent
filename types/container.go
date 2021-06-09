package types

import (
	"fmt"

	coretypes "github.com/projecteru2/core/types"
)

const (
	fieldPodname        = "ERU_POD"
	fieldNodename       = "ERU_NODE_NAME"
	fieldCoreIdentifier = "eru.coreid"
)

// Container define agent view container
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

// LogFieldExtra returns the extra field of log line
// currently it contains podname, nodename, coreid, and networks
// which user can know where this container is
// a sample:
// {
//   "podname": "testpod",
//   "nodename": "testnode",
//   "coreid": "b60d121b438a380c343d5ec3c2037564b82ffef3",
//   "networks_test_calico_pool1": "10.243.122.1",
//   "networks_test_calico_pool2": "10.233.0.1",
// }
func (c *Container) LogFieldExtra() map[string]string {
	extra := map[string]string{
		"podname":  c.Env[fieldPodname],
		"nodename": c.Env[fieldNodename],
		"coreid":   c.Labels[fieldCoreIdentifier],
	}
	for name, addr := range c.Networks {
		extra[fmt.Sprintf("networks_%s", name)] = addr
	}
	return extra
}
