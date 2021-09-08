package selfmon

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/agent/common"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func newMockSelfmon(t *testing.T, withETCD bool) *Selfmon {
	ctx := context.Background()
	config := &types.Config{
		HostName: "fake",
		Store:    common.MocksStore,
		Runtime:  common.MocksRuntime,
		KV:       common.MocksKV,
		Log: types.LogConfig{
			Stdout: true,
		},
		Etcd: coretypes.EtcdConfig{
			Machines:   []string{"127.0.0.1:2379"},
			Prefix:     "/selfmon-agent",
			LockPrefix: "__lock__/selfmon-agent",
		},
		GlobalConnectionTimeout: 5 * time.Second,
	}

	m, err := New(ctx, config)
	assert.Nil(t, err)

	if withETCD {
		etcd, err := meta.NewETCD(config.Etcd, t)
		assert.Nil(t, err)
		m.kv = etcd
	}

	return m
}

func TestCloseTwice(t *testing.T) {
	m := newMockSelfmon(t, false)
	defer m.Close()
	m.Close()
	m.Close()
	<-m.Exit()
}

func TestEmbeddedETCD(t *testing.T) {
	etcd, err := meta.NewETCD(coretypes.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/selfmon-agent",
		LockPrefix: "__lock__/selfmon-agent",
	}, t)
	assert.Nil(t, err)

	ctx := context.Background()

	_, un, err := etcd.StartEphemeral(ctx, "/test/key", 1*time.Second)
	assert.Nil(t, err)
	time.Sleep(5 * time.Second)
	un()

	_, _, err = etcd.StartEphemeral(ctx, "/test/key", 1*time.Second)
	assert.Nil(t, err)
}

func TestRegisterTwice(t *testing.T) {
	m1 := newMockSelfmon(t, false)
	m2 := newMockSelfmon(t, false)
	defer m1.Close()
	defer m2.Close()

	// make sure m1 and m2 are using the same embedded ETCD
	etcd, err := meta.NewETCD(coretypes.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/selfmon-agent",
		LockPrefix: "__lock__/selfmon-agent",
	}, t)
	assert.Nil(t, err)

	m1.kv = etcd
	m2.kv = etcd

	ctx := context.Background()
	i := 0

	go m1.WithActiveLock(ctx, func(ctx context.Context) {
		i = 1
		time.Sleep(3 * time.Second)
	})
	time.Sleep(time.Second)
	go m2.WithActiveLock(ctx, func(ctx context.Context) {
		i = 2
	})
	assert.Equal(t, i, 1)
	time.Sleep(5 * time.Second)
	assert.Equal(t, i, 2)
}

func TestRun(t *testing.T) {
	m := newMockSelfmon(t, true)
	defer m.Close()

	store := m.store.(*storemocks.MockStore)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set node "fake" as alive
	assert.Nil(t, store.SetNodeStatus(ctx, 0))

	go func() {
		assert.Nil(t, m.Run(ctx))
	}()
	time.Sleep(2 * time.Second)

	node, _ := store.GetNode(ctx, "fake")
	assert.Equal(t, node.Available, true)
	node, _ = store.GetNode(ctx, "faker")
	assert.Equal(t, node.Available, false)

	go store.StartNodeStatusStream()
	time.Sleep(2 * time.Second)

	node, _ = store.GetNode(ctx, "fake")
	assert.Equal(t, node.Available, false)
	node, _ = store.GetNode(ctx, "faker")
	assert.Equal(t, node.Available, true)

	m.Close()
}
