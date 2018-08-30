package types

import (
	"sync"

	"github.com/docker/docker/api/types/network"
	coretypes "github.com/projecteru2/core/types"
)

type Container struct {
	ID          string
	Pid         int
	Running     bool
	Healthy     bool
	Name        string
	EntryPoint  string
	Ident       string
	Version     string
	CPUNum      float64
	CPUQuota    int64
	CPUPeriod   int64
	Memory      int64
	Extend      map[string]string
	Publish     map[string][]string
	Networks    map[string]*network.EndpointSettings `json:"-"`
	HealthCheck *coretypes.HealthCheck
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
