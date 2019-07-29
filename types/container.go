package types

import (
	"sync"

	coretypes "github.com/projecteru2/core/types"
)

// Container define agent view container
type Container struct {
	coretypes.ContainerStatus
	ID          string
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
	sync.Mutex
	data map[string]bool
}

// Set set data to store
func (p *PrevCheck) Set(ID string, f bool) {
	p.Lock()
	defer p.Unlock()
	p.data[ID] = f
}

// Get get data from store
func (p *PrevCheck) Get(ID string) bool {
	p.Lock()
	defer p.Unlock()
	v, ok := p.data[ID]
	if !ok {
		return false
	}
	return v
}

// Del delete data from store
func (p *PrevCheck) Del(ID string) {
	p.Lock()
	defer p.Unlock()
	delete(p.data, ID)
}

// NewPrevCheck new a prevcheck obj
func NewPrevCheck() *PrevCheck {
	return &PrevCheck{
		sync.Mutex{},
		map[string]bool{},
	}
}
