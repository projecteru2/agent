package mocks

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/runtime"
	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/mock"
)

// eva a fake workload
type eva struct {
	ID         string
	Name       string
	EntryPoint string
	Pid        int
	Running    bool
	Healthy    bool
}

// Nerv a fake runtime
type Nerv struct {
	Runtime
	sync.Mutex
	workloads     sync.Map // map[string]*eva
	msgChan       chan *types.WorkloadEventMessage
	errChan       chan error
	daemonRunning bool
}

func (n *Nerv) init() {
	n.workloads = sync.Map{}
	n.workloads.Store("Rei", &eva{
		ID:         "Rei",
		Name:       "nerv_eva0_boiled",
		EntryPoint: "eva0",
		Pid:        12306,
		Running:    true,
		Healthy:    false,
	})
	n.workloads.Store("Shinji", &eva{
		ID:         "Shinji",
		Name:       "nerv_eva1_related",
		EntryPoint: "eva1",
		Pid:        12307,
		Running:    true,
		Healthy:    true,
	})
	n.workloads.Store("Asuka", &eva{
		ID:         "Asuka",
		Name:       "nerv_eva2_genius",
		EntryPoint: "eva2",
		Pid:        12308,
		Running:    false, // not yet
		Healthy:    false,
	})

	n.msgChan = make(chan *types.WorkloadEventMessage)
	n.errChan = make(chan error)
	n.daemonRunning = true
}

func (n *Nerv) withLock(f func()) {
	n.Lock()
	defer n.Unlock()
	f()
}

// FromTemplate returns a mock runtime instance created from template
func FromTemplate() runtime.Runtime {
	n := &Nerv{}
	n.init()
	n.On("AttachWorkload", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, ID string) io.Reader {
			return strings.NewReader("stdout\n")
		},
		func(ctx context.Context, ID string) io.Reader {
			return strings.NewReader("stderr\n")
		},
		nil,
	)
	n.On("CollectWorkloadMetrics", mock.Anything, mock.Anything).Return()
	n.On("ListWorkloadIDs", mock.Anything, mock.Anything).Return(func(ctx context.Context, filters map[string]string) []string {
		var IDs []string
		n.withLock(func() {
			n.workloads.Range(func(ID, workload interface{}) bool {
				IDs = append(IDs, ID.(string))
				return true
			})
		})
		return IDs
	}, nil)
	n.On("Events", mock.Anything, mock.Anything).Return(func(ctx context.Context, filters map[string]string) <-chan *types.WorkloadEventMessage {
		return n.msgChan
	}, func(ctx context.Context, filters map[string]string) <-chan error {
		return n.errChan
	})
	n.On("GetStatus", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string, checkHealth bool) *types.WorkloadStatus {
		var status *types.WorkloadStatus
		n.withLock(func() {
			v, ok := n.workloads.Load(ID)
			if !ok {
				status = &types.WorkloadStatus{ID: ID}
				return
			}
			workload := v.(*eva)
			status = &types.WorkloadStatus{
				ID:      workload.ID,
				Running: workload.Running,
				Healthy: workload.Healthy,
			}
		})
		return status
	}, nil)
	n.On("GetWorkloadName", mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string) string {
		workload, ok := n.workloads.Load(ID)
		if !ok {
			return ""
		}
		return workload.(*eva).Name
	}, nil)
	n.On("LogFieldsExtra", mock.Anything, mock.Anything).Return(map[string]string{}, nil)
	n.On("IsDaemonRunning", mock.Anything).Return(func(ctx context.Context) bool {
		return n.daemonRunning
	})
	n.On("Name").Return("NERV")

	return n
}

// StartEvents starts the events: Shinji 400%, Asuka starts, Asuka dies, Rei dies
func (n *Nerv) StartEvents() {
	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Shinji",
		Action: "400%",
	}

	n.withLock(func() {
		v, _ := n.workloads.Load("Asuka")
		asuka := v.(*eva)
		asuka.Running = true
		asuka.Healthy = true
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusStart,
	}
	time.Sleep(time.Second)

	n.withLock(func() {
		v, _ := n.workloads.Load("Asuka")
		asuka := v.(*eva)
		asuka.Running = false
		asuka.Healthy = false
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusDie,
	}
	time.Sleep(time.Second)

	n.withLock(func() {
		v, _ := n.workloads.Load("Rei")
		rei := v.(*eva)
		rei.Running = false
		rei.Healthy = false
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Rei",
		Action: common.StatusDie,
	}
}

// StartCustomEvent .
func (n *Nerv) StartCustomEvent(event *types.WorkloadEventMessage) {
	n.msgChan <- event
}

// SetDaemonRunning set `daemonRunning`
func (n *Nerv) SetDaemonRunning(status bool) {
	n.daemonRunning = status
}
