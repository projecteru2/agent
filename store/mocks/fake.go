package mocks

import (
	"context"
	"sync"

	"github.com/alphadose/haxmap"
	"github.com/projecteru2/core/log"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
)

// MockStore .
type MockStore struct {
	Store
	sync.Mutex
	workloadStatus *haxmap.Map[string, *types.WorkloadStatus] // map[string]*types.WorkloadStatus
	nodeStatus     *haxmap.Map[string, *types.NodeStatus]     // map[string]*types.NodeStatus
	nodeInfo       *haxmap.Map[string, *types.Node]           // map[string]*types.Node

	msgChan chan *types.NodeStatus
	errChan chan error
}

func (m *MockStore) init() {
	m.workloadStatus = haxmap.New[string, *types.WorkloadStatus]()
	m.nodeStatus = haxmap.New[string, *types.NodeStatus]()
	m.nodeInfo = haxmap.New[string, *types.Node]()
	m.msgChan = make(chan *types.NodeStatus)
	m.errChan = make(chan error)

	m.nodeInfo.Set("fake", &types.Node{
		Name:     "fake",
		Endpoint: "eva://127.0.0.1:6666",
	})
	m.nodeInfo.Set("faker", &types.Node{
		Name:     "faker",
		Endpoint: "eva://127.0.0.1:6667",
	})
}

// NewFakeStore returns a mock store instance created from mock
func NewFakeStore() store.Store {
	logger := log.WithFunc("fakestore")
	m := &MockStore{}
	m.init()
	m.On("GetNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.Node {
		m.Lock()
		defer m.Unlock()
		node, ok := m.nodeInfo.Get(nodename)
		if !ok {
			return nil
		}
		return &types.Node{
			Name:      node.Name,
			Available: node.Available,
		}
	}, nil)
	m.On("SetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, ttl int64) error {
		logger.Info(ctx, "set node status")
		nodename := "fake"
		m.Lock()
		defer m.Unlock()
		if status, ok := m.nodeStatus.Get(nodename); ok {
			status.Alive = true
		} else {
			m.nodeStatus.Set(nodename, &types.NodeStatus{
				Nodename: nodename,
				Alive:    true,
			})
		}
		return nil
	})
	m.On("GetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.NodeStatus {
		m.Lock()
		defer m.Unlock()
		if status, ok := m.nodeStatus.Get(nodename); ok {
			return &types.NodeStatus{
				Nodename: status.Nodename,
				Alive:    status.Alive,
			}
		}
		return &types.NodeStatus{
			Nodename: nodename,
			Alive:    false,
		}
	}, nil)
	m.On("SetWorkloadStatus", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, status *types.WorkloadStatus, ttl int64) error {
		logger.Infof(ctx, "set workload status: %+v\n", status)
		m.workloadStatus.Set(status.ID, status)
		return nil
	})
	m.On("GetIdentifier", mock.Anything).Return("fake-identifier")
	m.On("NodeStatusStream", mock.Anything).Return(func(ctx context.Context) <-chan *types.NodeStatus {
		return m.msgChan
	}, func(ctx context.Context) <-chan error {
		return m.errChan
	})
	m.On("ListPodNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{
		{
			Name: "fake",
		},
		{
			Name: "faker",
		},
	}, nil)

	return m
}

// GetMockWorkloadStatus returns the mock workload status by ID
func (m *MockStore) GetMockWorkloadStatus(ID string) *types.WorkloadStatus {
	status, ok := m.workloadStatus.Get(ID)
	if !ok {
		return nil
	}
	return status
}

// StartNodeStatusStream "faker" up, "fake" down.
func (m *MockStore) StartNodeStatusStream() {
	m.msgChan <- &types.NodeStatus{
		Nodename: "faker",
		Alive:    true,
	}
	m.msgChan <- &types.NodeStatus{
		Nodename: "fake",
		Alive:    false,
	}
}

// StopNodeStatusStream send an err to errChan.
func (m *MockStore) StopNodeStatusStream() {
	m.errChan <- common.ErrClosedSteam
}
