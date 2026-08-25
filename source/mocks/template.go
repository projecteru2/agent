package mocks

import (
	"context"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
)

type Nerv struct {
	Source
	sync.Mutex
	workloads     map[string]*source.Workload
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

	n.withLock(func() { n.workloads["Asuka"].Running = true })

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusStart,
	}
	time.Sleep(time.Second)

	n.withLock(func() { n.workloads["Asuka"].Running = false })

	n.msgChan <- &types.WorkloadEventMessage{
		ID:     "Asuka",
		Action: common.StatusDie,
	}
	time.Sleep(time.Second)

	n.withLock(func() { n.workloads["Rei"].Running = false })

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

// Attach makes Nerv a source.Attacher, so the workload manager pumps its logs.
func (n *Nerv) Attach(context.Context, string) (io.Reader, io.Reader, error) {
	return strings.NewReader("stdout\n"), strings.NewReader("stderr\n"), nil
}

func (n *Nerv) init() {
	// Rei probes a closed port, so it is running and never healthy
	n.workloads = map[string]*source.Workload{
		"Rei": {
			ID:      "Rei",
			Meta:    source.Meta{Appname: "nerv", Entrypoint: "eva0", Ident: "boiled", HealthCheck: &coretypes.HealthCheck{TCPPorts: []string{"1"}}},
			LocalIP: common.LocalIP,
			Running: true,
		},
		"Shinji": {
			ID:      "Shinji",
			Meta:    source.Meta{Appname: "nerv", Entrypoint: "eva1", Ident: "related"},
			Running: true,
		},
		"Asuka": {
			ID:   "Asuka",
			Meta: source.Meta{Appname: "nerv", Entrypoint: "eva2", Ident: "genius"},
		},
	}

	n.msgChan = make(chan *types.WorkloadEventMessage)
	n.errChan = make(chan error)
	n.daemonRunning = true
}

func (n *Nerv) snapshot(ID string) *source.Workload {
	var w source.Workload
	n.withLock(func() {
		if known, ok := n.workloads[ID]; ok {
			w = *known
		} else {
			w = source.Workload{ID: ID}
		}
	})
	return &w
}

func (n *Nerv) withLock(f func()) {
	n.Lock()
	defer n.Unlock()
	f()
}

func FromTemplate() source.Source {
	n := &Nerv{}
	n.init()
	n.On("List", mock.Anything).Return(func(ctx context.Context) []*source.Workload {
		var IDs []string
		n.withLock(func() { IDs = slices.Collect(maps.Keys(n.workloads)) })

		workloads := make([]*source.Workload, 0, len(IDs))
		for _, ID := range IDs {
			workloads = append(workloads, n.snapshot(ID))
		}
		return workloads
	}, nil)
	n.On("Get", mock.Anything, mock.Anything).Return(func(ctx context.Context, ID string) *source.Workload {
		return n.snapshot(ID)
	}, nil)
	n.On("Events", mock.Anything).Return(func(ctx context.Context) <-chan *types.WorkloadEventMessage {
		return n.msgChan
	}, func(ctx context.Context) <-chan error {
		return n.errChan
	})
	n.On("Alive", mock.Anything).Return(func(ctx context.Context) bool {
		var running bool
		n.withLock(func() { running = n.daemonRunning })
		return running
	})

	return n
}
