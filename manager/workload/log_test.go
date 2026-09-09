package workload

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/types"
)

const (
	streamTimeout = 10 * time.Second
	streamPoll    = time.Millisecond
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /log/{$}", handler)
	server := &http.Server{Handler: mux}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer func() {
		assert.NoError(t, server.Shutdown(t.Context()))
		assert.Equal(t, http.ErrServerClosed, <-served)
	}()

	reqCtx, reqCancel := context.WithTimeout(t.Context(), streamTimeout)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+listener.Addr().String()+"/log/?app=nerv", nil)
	assert.Nil(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()

	require.Eventually(t, func() bool {
		return subscriberCount(manager.logBroadcaster, "nerv") == 1
	}, streamTimeout, streamPoll, "the log stream never subscribed")

	for _, data := range []string{"data0", "data1"} {
		manager.logBroadcaster.broadcast(t.Context(), &types.Log{
			ID:         "Rei",
			Name:       "nerv",
			Type:       "stdout",
			EntryPoint: "eva0",
			Data:       data,
		})
	}

	reader := bufio.NewReader(resp.Body)
	for range 2 {
		line, err := reader.ReadBytes('\n')
		assert.Nil(t, err)
		t.Log(string(line))
	}

	logCancel()
	require.Eventually(t, func() bool {
		return subscriberCount(manager.logBroadcaster, "nerv") == 0
	}, streamTimeout, streamPoll, "the canceled log stream never detached")

	manager.logBroadcaster.broadcast(t.Context(), &types.Log{
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
	assert.Equal(t, counted+3, testutil.ToFloat64(droppedBySubscriber))
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

func subscriberCount(l *logBroadcaster, app string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.subscribersMap[app])
}
