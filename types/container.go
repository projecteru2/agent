package types

import (
	"time"

	"github.com/patrickmn/go-cache"
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

// PrevCheck store healthcheck data
type PrevCheck struct {
	data *cache.Cache
}

// Set set data to store
func (p *PrevCheck) Set(ID string, f bool) {
	p.data.Set(ID, f, cache.DefaultExpiration)
}

// Get get data from store
func (p *PrevCheck) Get(ID string) (bool, bool) {
	v, ok := p.data.Get(ID)
	if !ok {
		return false, false
	}
	return v.(bool), true
}

// Del delete data from store
func (p *PrevCheck) Del(ID string) {
	p.data.Delete(ID)
}

// NewPrevCheck new a prevcheck obj
func NewPrevCheck(config *Config) *PrevCheck {
	return &PrevCheck{
		cache.New(time.Duration(config.HealthCheckCacheTTL)*time.Second, 60*time.Minute),
	}
}
