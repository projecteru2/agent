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

type eva struct {
	ID         string
	Name       string
	EntryPoint string
	Pid        int
	Running    bool
	Healthy    bool
}

type Nerv struct {
	Runtime
	sync.Mutex
	workloads     map[string]*eva
	msgChan       chan *types.WorkloadEventMessage
	errChan       chan error
	daemonRunning bool
}

// StartEvents starts the events: Shinji 400%, Asuka starts, Asuka dies, Rei dies
func (n *Nerv) StartEvents() {
	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Shinji",
		Action: "400%",
	}

	n.withLock(func() {
		asuka := n.workloads["Asuka"]
		asuka.Running = true
		asuka.Healthy = true
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusStart,
	}
	time.Sleep(time.Second)

	n.withLock(func() {
		asuka := n.workloads["Asuka"]
		asuka.Running = false
		asuka.Healthy = false
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusDie,
	}
	time.Sleep(time.Second)

	n.withLock(func() {
		rei := n.workloads["Rei"]
		rei.Running = false
		rei.Healthy = false
	})

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Rei",
		Action: common.StatusDie,
	}
}

func (n *Nerv) StartCustomEvent(event *types.WorkloadEventMessage) {
	n.msgChan <- event
}

func (n *Nerv) SetDaemonRunning(status bool) {
	n.withLock(func() { n.daemonRunning = status })
}

func (n *Nerv) init() {
	n.workloads = map[string]*eva{
		"Rei": {
			ID:         "Rei",
			Name:       "nerv_eva0_boiled",
			EntryPoint: "eva0",
			Pid:        12306,
			Running:    true,
			Healthy:    false,
		},
		"Shinji": {
			ID:         "Shinji",
			Name:       "nerv_eva1_related",
			EntryPoint: "eva1",
			Pid:        12307,
			Running:    true,
			Healthy:    true,
		},
		"Asuka": {
			ID:         "Asuka",
			Name:       "nerv_eva2_genius",
			EntryPoint: "eva2",
			Pid:        12308,
			Running:    false,
			Healthy:    false,
		},
	}

	n.msgChan = make(chan *types.WorkloadEventMessage)
	n.errChan = make(chan error)
	n.daemonRunning = true
}

func (n *Nerv) withLock(f func()) {
	n.Lock()
	defer n.Unlock()
	f()
}

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
			for ID := range n.workloads {
				IDs = append(IDs, ID)
			}
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
			workload, ok := n.workloads[ID]
			if !ok {
				status = &types.WorkloadStatus{ID: ID}
				return
			}
			status = &types.WorkloadStatus{
				ID:      workload.ID,
				Running: workload.Running,
				Healthy: workload.Healthy,
			}
		})
		return status
	}, nil)
	n.On("GetWorkloadName", mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string) string {
		var name string
		n.withLock(func() {
			if workload, ok := n.workloads[ID]; ok {
				name = workload.Name
			}
		})
		return name
	}, nil)
	n.On("LogFieldsExtra", mock.Anything, mock.Anything).Return(map[string]string{}, nil)
	n.On("IsDaemonRunning", mock.Anything).Return(func(ctx context.Context) bool {
		var running bool
		n.withLock(func() { running = n.daemonRunning })
		return running
	})
	n.On("Name").Return("NERV")

	return n
}
