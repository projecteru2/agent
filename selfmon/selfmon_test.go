package selfmon

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	pb "github.com/projecteru2/core/rpc/gen"
	coremeta "github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/selfmon/mocks"
	"github.com/projecteru2/agent/types"
)

func TestCloseTwice(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()
	m.rpc.(*mocks.CoreRPCClient).On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{}, nil)
	m.Close()
	m.Close()
	<-m.Exit()
}

func TestRun(t *testing.T) {
	m, cancel := newTestSelfmon(t)

	rpc, ok := m.rpc.(*mocks.CoreRPCClient)
	require.True(t, ok)
	rpc.On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{
		Nodes: []*pb.Node{
			{
				Name:     "foo",
				Endpoint: "host:port",
			},
		},
	}, nil).Once()
	rpc.On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{}, nil)
	rpc.On("SetNode", mock.Anything, mock.Anything).Return(&pb.Node{}, nil)
	defer rpc.AssertExpectations(t)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Run()
	}()

	// Makes it as an active selfmon.
	m.active.Set()
	time.Sleep(time.Second)

	cancel()
	wg.Wait()
}

func TestRegister(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()

	unregister0, err := m.Register()
	require.NoError(t, err)
	require.NotNil(t, unregister0)

	unregister1, err := m.Register()
	require.NoError(t, err)
	require.NotNil(t, unregister1)

	unregister0()

	time.Sleep(time.Second)
	unregister1()
}

func newTestSelfmon(t *testing.T) (*Selfmon, func()) {
	config := &types.Config{
		Etcd: coretypes.EtcdConfig{
			Machines:   []string{"127.0.0.1:2379"},
			Prefix:     "/selfmon-agent",
			LockPrefix: "__lock__/selfmon-agent",
		},
	}

	m := &Selfmon{}
	m.config = config
	m.exit.C = make(chan struct{}, 1)
	m.rpc = &mocks.CoreRPCClient{}

	// Uses an embedded one instead of the real one.
	etcd, err := coremeta.NewETCD(config.Etcd, t)
	require.NoError(t, err)
	m.etcd = etcd

	m.rpc.(*mocks.CoreRPCClient).On("NodeStatusStream", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("mock"))
	m.rpc.(*mocks.CoreRPCClient).On("GetNodeStatus", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("mock"))
	return m, m.Close
}
