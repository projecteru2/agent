package mocks

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
	"github.com/stretchr/testify/mock"
)

// MockStore .
type MockStore struct {
	Store
	sync.Mutex
	workloadStatus sync.Map // map[string]*types.WorkloadStatus
	nodeStatus     sync.Map // map[string]*types.NodeStatus
	nodeInfo       sync.Map // map[string]*types.Node

	msgChan chan *types.NodeStatus
	errChan chan error
}

func (m *MockStore) init() {
	m.workloadStatus = sync.Map{}
	m.nodeStatus = sync.Map{}
	m.msgChan = make(chan *types.NodeStatus)
	m.errChan = make(chan error)

	m.nodeInfo = sync.Map{}
	m.nodeInfo.Store("fake", &types.Node{
		Name:     "fake",
		Endpoint: "eva://127.0.0.1:6666",
	})
	m.nodeInfo.Store("faker", &types.Node{
		Name:     "faker",
		Endpoint: "eva://127.0.0.1:6667",
	})
}

// FromTemplate returns a mock store instance created from template
func FromTemplate() store.Store {
	m := &MockStore{}
	m.init()
	m.On("GetNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.Node {
		m.Lock()
		defer m.Unlock()
		v, ok := m.nodeInfo.Load(nodename)
		if !ok {
			return nil
		}
		node := v.(*types.Node)
		return &types.Node{
			Name:      node.Name,
			Available: node.Available,
		}
	}, nil)
	m.On("SetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, ttl int64) error {
		fmt.Printf("[MockStore] set node status\n")
		nodename := "fake"
		m.Lock()
		defer m.Unlock()
		if status, ok := m.nodeStatus.Load(nodename); ok {
			status.(*types.NodeStatus).Alive = true
		} else {
			m.nodeStatus.Store(nodename, &types.NodeStatus{
				Nodename: nodename,
				Alive:    true,
			})
		}
		return nil
	})
	m.On("GetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.NodeStatus {
		if status, ok := m.nodeStatus.Load(nodename); ok {
			return status.(*types.NodeStatus)
		}
		return &types.NodeStatus{
			Nodename: nodename,
			Alive:    false,
		}
	}, nil)
	m.On("SetWorkloadStatus", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, status *types.WorkloadStatus, ttl int64) error {
		fmt.Printf("[MockStore] set workload status: %+v\n", status)
		m.workloadStatus.Store(status.ID, status)
		return nil
	})
	m.On("GetIdentifier", mock.Anything).Return("fake-identifier")
	m.On("SetNode", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, status bool) error {
		fmt.Printf("[MockStore] set node %s as status: %v\n", node, status)
		m.Lock()
		defer m.Unlock()
		if nodeInfo, ok := m.nodeInfo.Load(node); ok {
			nodeInfo.(*types.Node).Available = status
		} else {
			m.nodeInfo.Store(node, &types.Node{
				Name:      node,
				Available: status,
			})
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
	status, ok := m.workloadStatus.Load(ID)
	if !ok {
		return nil
	}
	return status.(*types.WorkloadStatus)
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
