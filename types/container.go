package types

import (
	coretypes "github.com/projecteru2/core/types"
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
	HealthCheck *coretypes.HealthCheck
	LocalIP     string `json:"-"`
}
