package node

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
)

func TestRun(t *testing.T) {
	manager := newMockNodeManager(t)
	store := manager.store.(*storemocks.MockStore)

	ctx, cancel := context.WithTimeout(t.Context(), time.Duration(manager.config.HeartbeatInterval*3)*time.Second)
	defer cancel()

	status, err := store.GetNodeStatus(ctx, "fake")
	assert.Nil(t, err)
	assert.Equal(t, status.Alive, false)

	go func() {
		time.Sleep(time.Duration(manager.config.HeartbeatInterval*2) * time.Second)
		status, err := store.GetNodeStatus(ctx, "fake")
		assert.Nil(t, err)
		assert.Equal(t, status.Alive, true)
	}()

	assert.Nil(t, manager.Run(ctx))
}

func TestExitLeavesTheDeleteAsTheLastNodeStatusWrite(t *testing.T) {
	statusStore := newOrderedStatusStore()
	manager := &Manager{
		config: &types.Config{GlobalConnectionTimeout: time.Second},
		store:  statusStore,
	}

	reportDone := make(chan error, 1)
	go func() { reportDone <- manager.setNodeStatus(t.Context(), 180) }()
	<-statusStore.entered

	exitDone := make(chan error, 1)
	go func() { exitDone <- manager.Exit(t.Context()) }()
	close(statusStore.release)

	assert.NoError(t, <-reportDone)
	assert.NoError(t, <-exitDone)
	assert.NoError(t, manager.setNodeStatus(t.Context(), 180))
	assert.Equal(t, []int64{180, -1}, statusStore.statusWrites())
}

func TestExitLeavesTheRemovalBudgetToTheStore(t *testing.T) {
	statusStore := newBudgetStatusStore()
	manager := &Manager{
		config: &types.Config{GlobalConnectionTimeout: time.Second},
		store:  statusStore,
	}

	reportDone := make(chan error, 1)
	go func() { reportDone <- manager.setNodeStatus(t.Context(), 180) }()
	<-statusStore.entered

	exitDone := make(chan error, 1)
	go func() { exitDone <- manager.Exit(t.Context()) }()
	close(statusStore.release)

	assert.NoError(t, <-reportDone)
	assert.NoError(t, <-exitDone)
}

func TestExitRemovesNodeStatus(t *testing.T) {
	manager := newMockNodeManager(t)
	store := manager.store.(*storemocks.MockStore)

	manager.nodeStatusReport(t.Context())
	status, err := store.GetNodeStatus(t.Context(), "fake")
	assert.NoError(t, err)
	assert.True(t, status.Alive)

	assert.NoError(t, manager.Exit(t.Context()))
	status, err = store.GetNodeStatus(t.Context(), "fake")
	assert.NoError(t, err)
	assert.False(t, status.Alive)
}

func TestExitRetriesFailedNodeStatusRemoval(t *testing.T) {
	statusStore := &retryingStatusStore{}
	manager := &Manager{
		config: &types.Config{GlobalConnectionTimeout: time.Second},
		store:  statusStore,
	}

	assert.Error(t, manager.Exit(t.Context()))
	assert.NoError(t, manager.Exit(t.Context()))
	assert.NoError(t, manager.setNodeStatus(t.Context(), 180))
	assert.Equal(t, 2, statusStore.calls)
}

func newMockNodeManager(t *testing.T) *Manager {
	config := &types.Config{
		HostName:          "fake",
		HeartbeatInterval: 2,
		CheckOnlyMine:     false,
		Store:             common.MocksStore,
		Runtimes:          types.RuntimesConfig{Mocks: &types.MocksConfig{}},
		Log: types.LogConfig{
			Stdout: true,
		},
		HealthCheck: types.HealthCheckConfig{
			Interval: 10,
			Timeout:  5,
			CacheTTL: 300,
		},
		GlobalConnectionTimeout: 5 * time.Second,
	}

	m, err := NewManager(t.Context(), config)
	assert.Nil(t, err)
	return m
}

type orderedStatusStore struct {
	store.Store
	mutex   sync.Mutex
	entered chan struct{}
	release chan struct{}
	writes  []int64
}

func (s *orderedStatusStore) SetNodeStatus(ctx context.Context, ttl int64) error {
	if ttl > 0 {
		close(s.entered)
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.writes = append(s.writes, ttl)
	return nil
}

func (s *orderedStatusStore) statusWrites() []int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return slices.Clone(s.writes)
}

func newOrderedStatusStore() *orderedStatusStore {
	return &orderedStatusStore{entered: make(chan struct{}), release: make(chan struct{})}
}

type budgetStatusStore struct {
	store.Store
	entered chan struct{}
	release chan struct{}
}

func (s *budgetStatusStore) SetNodeStatus(ctx context.Context, ttl int64) error {
	if ttl >= 0 {
		close(s.entered)
		<-s.release
		return nil
	}
	if _, ok := ctx.Deadline(); ok {
		return errors.New("the removal inherited an outer deadline")
	}
	return nil
}

func newBudgetStatusStore() *budgetStatusStore {
	return &budgetStatusStore{entered: make(chan struct{}), release: make(chan struct{})}
}

type retryingStatusStore struct {
	store.Store
	calls int
}

func (s *retryingStatusStore) SetNodeStatus(context.Context, int64) error {
	s.calls++
	if s.calls == 1 {
		return errors.New("temporary store failure")
	}
	return nil
}
