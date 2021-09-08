package mocks

import (
	"context"
	"strings"
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
	workloads     map[string]*eva
	msgChan       chan *types.WorkloadEventMessage
	errChan       chan error
	daemonRunning bool
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
			Running:    false, // not yet
			Healthy:    false,
		},
	}

	n.msgChan = make(chan *types.WorkloadEventMessage)
	n.errChan = make(chan error)
	n.daemonRunning = true
}

// FromTemplate returns a mock runtime instance created from template
func FromTemplate() runtime.Runtime {
	n := &Nerv{}
	n.init()
	n.On("AttachWorkload", mock.Anything, mock.Anything).Return(strings.NewReader("stdout\n"), strings.NewReader("stderr\n"), nil)
	n.On("CollectWorkloadMetrics", mock.Anything, mock.Anything).Return()
	n.On("ListWorkloadIDs", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, all bool, filters []types.KV) []string {
		var IDs []string
		for ID, workload := range n.workloads {
			if all || workload.Running {
				IDs = append(IDs, ID)
			}
		}
		return IDs
	}, nil)
	n.On("Events", mock.Anything, mock.Anything).Return(func(ctx context.Context, filters []types.KV) <-chan *types.WorkloadEventMessage {
		return n.msgChan
	}, func(ctx context.Context, filters []types.KV) <-chan error {
		return n.errChan
	})
	n.On("GetStatus", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string, checkHealth bool) *types.WorkloadStatus {
		workload := n.workloads[ID]
		return &types.WorkloadStatus{
			ID:      workload.ID,
			Running: workload.Running,
			Healthy: workload.Healthy,
		}
	}, nil)
	n.On("GetWorkloadName", mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string) string {
		return n.workloads[ID].Name
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

	n.workloads["Asuka"].Running = true
	n.workloads["Asuka"].Healthy = true
	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusStart,
	}
	time.Sleep(time.Second)

	n.workloads["Asuka"].Running = false
	n.workloads["Asuka"].Healthy = false
	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusDie,
	}
	time.Sleep(time.Second)

	n.workloads["Rei"].Running = false
	n.workloads["Rei"].Healthy = false
	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Rei",
		Action: common.StatusDie,
	}
}

// SetDaemonRunning set `daemonRunning`
func (n *Nerv) SetDaemonRunning(status bool) {
	n.daemonRunning = status
}
