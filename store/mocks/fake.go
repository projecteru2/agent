package mocks

import (
	"context"
	"fmt"
	"slices"
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

func NewFakeStore() store.Store {
	logger := log.WithFunc("mocks.NewFakeStore")
	m := &MockStore{}
	m.init()
	m.On("GetNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, nodename string) *types.Node {
		m.Lock()
		defer m.Unlock()
		node, ok := m.nodeInfo[nodename]
		if !ok {
			return nil
		}
		return &types.Node{Endpoint: node.Endpoint}
	}, func(ctx context.Context, nodename string) error {
		m.Lock()
		defer m.Unlock()
		if _, ok := m.nodeInfo[nodename]; !ok {
			return fmt.Errorf("node %s not found", nodename)
		}
		return nil
	})
	m.On("SetNodeStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, ttl int64) error {
		logger.Info(ctx, "set node status")
		nodename := "fake"
		m.Lock()
		defer m.Unlock()
		if ttl < 0 {
			delete(m.nodeStatus, nodename)
			return nil
		}
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
	m.On("SetWorkloadStatus", mock.Anything, mock.Anything).Return(func(ctx context.Context, status *types.WorkloadStatus) error {
		logger.Infof(ctx, "set workload status: %+v", status)
		m.Lock()
		defer m.Unlock()
		m.workloadStatus[status.ID] = status
		return nil
	})
	m.On("GetIdentifier", mock.Anything).Return("fake-identifier")

	return m
}

func (m *MockStore) GetMockNodeStatus(nodename string) *types.NodeStatus {
	m.Lock()
	defer m.Unlock()
	if status, ok := m.nodeStatus[nodename]; ok {
		return &types.NodeStatus{Nodename: status.Nodename, Alive: status.Alive}
	}
	return &types.NodeStatus{Nodename: nodename}
}

func (m *MockStore) GetMockWorkloadStatus(ID string) *types.WorkloadStatus {
	m.Lock()
	defer m.Unlock()
	return m.workloadStatus[ID]
}

func (m *MockStore) ListRunningWorkloadIDs(context.Context) ([]string, error) {
	m.Lock()
	defer m.Unlock()
	running := make([]string, 0, len(m.workloadStatus))
	for ID, status := range m.workloadStatus {
		if status.Running {
			running = append(running, ID)
		}
	}
	slices.Sort(running)
	return running, nil
}

func (m *MockStore) init() {
	m.workloadStatus = map[string]*types.WorkloadStatus{}
	m.nodeStatus = map[string]*types.NodeStatus{}
	m.nodeInfo = map[string]*types.Node{
		"fake": {Endpoint: "eva://127.0.0.1:6666"},
	}
}
