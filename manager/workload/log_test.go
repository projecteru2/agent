package workload

import (
	"bufio"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/types"
)

func TestLogBroadcaster(t *testing.T) {
	manager := newMockWorkloadManager(t)

	logCtx, logCancel := context.WithCancel(t.Context())
	defer logCancel()

	handler := func(w http.ResponseWriter, req *http.Request) {
		app := req.URL.Query().Get("app")
		if app == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		if hijack, ok := w.(http.Hijacker); ok {
			conn, buf, err := hijack.Hijack()
			if err != nil {
				return
			}
			defer conn.Close()
			manager.PullLog(logCtx, app, buf)
		}
	}
	server := &http.Server{Addr: ":12310"}
	defer func() { _ = server.Shutdown(context.Background()) }()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /log/{$}", handler)
		server.Handler = mux
		assert.Equal(t, server.ListenAndServe(), http.ErrServerClosed)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 7*time.Second)
	defer cancel()

	go func() {
		time.Sleep(3 * time.Second)
		manager.logBroadcaster.broadcast(ctx, &types.Log{
			ID:         "Rei",
			Name:       "nerv",
			Type:       "stdout",
			EntryPoint: "eva0",
			Data:       "data0",
		})
		manager.logBroadcaster.broadcast(ctx, &types.Log{
			ID:         "Rei",
			Name:       "nerv",
			Type:       "stdout",
			EntryPoint: "eva0",
			Data:       "data1",
		})
	}()

	time.Sleep(time.Second)

	reqCtx, reqCancel := context.WithTimeout(ctx, 3*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", "http://127.0.0.1:12310/log/?app=nerv", nil)
	assert.Nil(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for range 2 {
		line, err := reader.ReadBytes('\n')
		assert.Nil(t, err)
		t.Log(string(line))
	}

	logCancel()
	time.Sleep(time.Second)

	manager.logBroadcaster.broadcast(ctx, &types.Log{
		ID:         "Rei",
		Name:       "nerv",
		Type:       "stdout",
		EntryPoint: "eva0",
		Data:       "data1",
	})
	manager.logBroadcaster.mu.RLock()
	defer manager.logBroadcaster.mu.RUnlock()
	assert.Empty(t, manager.logBroadcaster.subscribersMap)
}

func TestSubscriberSendDropsWhatAStalledClientCannotTake(t *testing.T) {
	sub := &subscriber{lines: make(chan []byte, 2)}
	counted := testutil.ToFloat64(droppedBySubscriber)

	for range 5 {
		sub.send([]byte("line"))
	}

	assert.Len(t, sub.lines, 2)
	assert.Equal(t, int64(3), sub.dropped.Load())
	assert.InDelta(t, counted+3, testutil.ToFloat64(droppedBySubscriber), 0)
}

func TestBroadcastDoesNotBlockOnAStalledSubscriber(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	stalled := &subscriber{ctx: ctx, cancel: cancel, lines: make(chan []byte, 1)}
	broadcaster := newLogBroadcaster()
	broadcaster.subscribersMap["nerv"] = map[string]*subscriber{"stalled": stalled}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			broadcaster.broadcast(ctx, &types.Log{Name: "nerv", Data: "data"})
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("broadcast blocked on a subscriber that stopped reading")
	}
	assert.Positive(t, stalled.dropped.Load())
}
