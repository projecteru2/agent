package mocks

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/agent/store"
	"github.com/projecteru2/agent/types"
)

type MockStore struct {
	Store
	sync.Mutex
	workloadStatus map[string]*types.WorkloadStatus
	nodeStatus     map[string]*types.NodeStatus
	nodeInfo       map[string]*types.Node
}

func (m *MockStore) GetMockWorkloadStatus(ID string) *types.WorkloadStatus {
	m.Lock()
	defer m.Unlock()
	return m.workloadStatus[ID]
}

func (m *MockStore) init() {
	m.workloadStatus = map[string]*types.WorkloadStatus{}
	m.nodeStatus = map[string]*types.NodeStatus{}
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

func NewFakeStore() store.Store {
	logger := log.WithFunc("fakestore")
	m := &MockStore{}
	m.init()
	m.On("GetNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.Node {
		m.Lock()
		defer m.Unlock()
		node, ok := m.nodeInfo[nodename]
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
		m.Lock()
		defer m.Unlock()
		if status, ok := m.nodeStatus[nodename]; ok {
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
		logger.Infof(ctx, "set workload status: %+v", status)
		m.Lock()
		defer m.Unlock()
		m.workloadStatus[status.ID] = status
		return nil
	})
	m.On("GetIdentifier", mock.Anything).Return("fake-identifier")

	return m
}
