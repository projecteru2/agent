package mocks

import (
	"context"
	"fmt"

	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
	"github.com/stretchr/testify/mock"
)

// MockStore .
type MockStore struct {
	Store
	workloadStatus map[string]*types.WorkloadStatus
	nodeStatus     map[string]*types.NodeStatus
	nodeInfo       map[string]*types.Node
	msgChan        chan *types.NodeStatus
	errChan        chan error
}

func (m *MockStore) init() {
	m.workloadStatus = map[string]*types.WorkloadStatus{}
	m.nodeStatus = map[string]*types.NodeStatus{}
	m.msgChan = make(chan *types.NodeStatus)
	m.errChan = make(chan error)

	m.nodeInfo = map[string]*types.Node{
		"fake": {
			Name:     "fake",
			Endpoint: "eva://127.0.0.1:6666",
		},
		"faker": {
			Name:     "faker",
			Endpoint: "eva://127.0.0.1:6667",
		},
	}
}

// FromTemplate returns a mock store instance created from template
func FromTemplate() store.Store {
	m := &MockStore{}
	m.init()
	m.On("GetNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.Node {
		return m.nodeInfo[nodename]
	}, nil)
	m.On("SetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, ttl int64) error {
		fmt.Printf("[MockStore] set node status\n")
		nodename := "fake"
		if status, ok := m.nodeStatus[nodename]; ok {
			status.Alive = true
		} else {
			m.nodeStatus[nodename] = &types.NodeStatus{
				Nodename: nodename,
				Alive:    true,
			}
		}
		return nil
	})
	m.On("GetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.NodeStatus {
		if status, ok := m.nodeStatus[nodename]; ok {
			return status
		}
		return &types.NodeStatus{
			Nodename: nodename,
			Alive:    false,
		}
	}, nil)
	m.On("SetWorkloadStatus", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, status *types.WorkloadStatus, ttl int64) error {
		fmt.Printf("[MockStore] set workload status: %+v\n", status)
		m.workloadStatus[status.ID] = status
		return nil
	})
	m.On("GetIdentifier", mock.Anything).Return("fake-identifier")
	m.On("SetNode", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, status bool) error {
		fmt.Printf("[MockStore] set node %s as status: %v\n", node, status)
		if nodeInfo, ok := m.nodeInfo[node]; ok {
			nodeInfo.Available = status
		} else {
			m.nodeInfo[node] = &types.Node{
				Name:      node,
				Available: status,
			}
		}
		return nil
	})
	m.On("ListPodNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{
		{
			Name: "fake",
		},
		{
			Name: "faker",
		},
	}, nil)
	m.On("NodeStatusStream", mock.Anything).Return(func(ctx context.Context) <-chan *types.NodeStatus {
		return m.msgChan
	}, func(ctx context.Context) <-chan error {
		return m.errChan
	})

	return m
}

// GetMockWorkloadStatus returns the mock workload status by ID
func (m *MockStore) GetMockWorkloadStatus(ID string) *types.WorkloadStatus {
	return m.workloadStatus[ID]
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
