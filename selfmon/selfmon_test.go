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

func TestRunFailedAsInvalidProtobufNode(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	m.rpc.(*mocks.CoreRPCClient).On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{}, nil)
	m.nodes.Store("foo", "bar")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.Run()
	}()

	cancel()
	wg.Wait()
}

func TestRun(t *testing.T) {
	m, cancel := newTestSelfmon(t)

	rpc, ok := m.rpc.(*mocks.CoreRPCClient)
	require.True(t, ok)
	rpc.On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{
		Nodes: []*pb.Node{&pb.Node{
			Name:     "foo",
			Endpoint: "host:port",
		}},
	}, nil).Once()
	rpc.On("ListPodNodes", mock.Anything, mock.Anything).Return(&pb.Nodes{}, nil)
	rpc.On("SetNode", mock.Anything, mock.Anything).Return(&pb.Node{}, nil)
	defer rpc.AssertExpectations(t)

	dec, ok := m.detector.(*mocks.Detector)
	require.True(t, ok)
	dec.On("Detect", mock.Anything).Return(fmt.Errorf("connecting failed"))
	defer dec.AssertExpectations(t)

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

func TestReportFailedAsInactive(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()

	m.deads.Store("foo", &pb.Node{})
	m.deads.Store("bar", &pb.Node{})
	m.report()
	assertEmptyMap(t, &m.deads)
}

func TestReportFailedAsRPCError(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()

	rpc, ok := m.rpc.(*mocks.CoreRPCClient)
	require.True(t, ok)
	rpc.On("SetNode", mock.Anything, mock.Anything).Return(&pb.Node{}, fmt.Errorf("err"))

	m.active.Set()
	m.deads.Store("foo", &pb.Node{})
	m.deads.Store("bar", &pb.Node{})
	m.report()
	assertEmptyMap(t, &m.deads)
}

func TestReportFailedAsInvalidDeadValue(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()

	m.deads.Store("foo", 1)
	m.deads.Store("bar", 1)
	m.report()
	assertEmptyMap(t, &m.deads)
}

func TestReportFailedAsInvalidDeadKey(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()

	m.deads.Store(1, &pb.Node{})
	m.deads.Store(2, &pb.Node{})
	m.report()
	assertEmptyMap(t, &m.deads)
}

func TestParseEndpointFailed(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()
	host, err := m.parseEndpoint("%z")
	require.Error(t, err)
	require.Equal(t, "", host)
}

func TestParseEndpoint(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	defer cancel()
	host, err := m.parseEndpoint("virt-grpc://10.22.12.43:9697")
	require.NoError(t, err)
	require.Equal(t, "10.22.12.43:9697", host)
}

func TestWatchFailedAsListPodNodesError(t *testing.T) {
	m, cancel := newTestSelfmon(t)
	m.rpc.(*mocks.CoreRPCClient).On("ListPodNodes", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("err"))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.watch()
	}()

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

func assertEmptyMap(t *testing.T, m *sync.Map) {
	m.Range(func(k, _ interface{}) bool {
		require.FailNow(t, "expects there's no any item")
		return false
	})
}

func newTestSelfmon(t *testing.T) (*Selfmon, func()) {
	config := &types.Config{
		Etcd: coretypes.EtcdConfig{
			Machines:   []string{"127.0.0.1:2379"},
			Prefix:     "/selfmon-agent",
			LockPrefix: "__lock__/selfmon-agent",
		},
	}

	m, err := New(config)
	require.NoError(t, err)
	m.rpc = &mocks.CoreRPCClient{}
	m.checkInterval = time.Microsecond * 100
	m.detector = &mocks.Detector{}

	// Uses an embedded one instead of the real one.
	m.etcd, err = coremeta.NewETCD(config.Etcd, true)
	require.NoError(t, err)

	return m, func() {
		m.etcd.TerminateEmbededStorage()
		m.Close()
	}
}
