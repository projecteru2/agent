package workload

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sourcemocks "github.com/projecteru2/agent/source/mocks"
	storemocks "github.com/projecteru2/agent/store/mocks"
)

func TestEvent(t *testing.T) {
	ctx := t.Context()

	manager := newMockWorkloadManager(t)
	src := manager.source.(*sourcemocks.Nerv)
	store := manager.store.(*storemocks.MockStore)
	assert.Nil(t, manager.initWorkloadStatus(ctx))
	assertInitStatus(t, store)

	go manager.monitor(ctx)

	go src.StartEvents()
	time.Sleep(5 * time.Second)

	assert.Equal(t, store.GetMockWorkloadStatus("Asuka"), wantStatus("Asuka", "eva2", false, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Rei"), wantStatus("Rei", "eva0", false, false))
	assert.Equal(t, store.GetMockWorkloadStatus("Shinji"), wantStatus("Shinji", "eva1", true, true))
}

func TestSerialQueueRunsOneKeyInSubmissionOrder(t *testing.T) {
	queue := newSerialQueue()

	var mu sync.Mutex
	var order []int
	done := make(chan struct{})

	for i := range 50 {
		queue.Go("Rei", func() {
			mu.Lock()
			order = append(order, i)
			if len(order) == 50 {
				close(done)
			}
			mu.Unlock()
		})
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the queue to drain")
	}

	want := make([]int, 50)
	for i := range want {
		want[i] = i
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, want, order)
}

func TestSerialQueueDoesNotMakeOneKeyWaitForAnother(t *testing.T) {
	queue := newSerialQueue()

	blocked := make(chan struct{})
	queue.Go("Rei", func() { <-blocked })
	defer close(blocked)

	ran := make(chan struct{})
	queue.Go("Shinji", func() { close(ran) })

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("a busy key blocked another one")
	}
}
