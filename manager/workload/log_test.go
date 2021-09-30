package workload

import (
	"bufio"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/projecteru2/agent/types"

	"github.com/bmizerany/pat"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogBroadcaster(t *testing.T) {
	manager := newMockWorkloadManager(t)
	logrus.SetLevel(logrus.DebugLevel)

	logCtx, logCancel := context.WithCancel(context.Background())
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

	go func() {
		restfulAPIServer := pat.New()
		restfulAPIServer.Add("GET", "/log/", http.HandlerFunc(handler))
		server.Handler = restfulAPIServer
		assert.Equal(t, server.ListenAndServe(), http.ErrServerClosed)
	}()

	go func() {
		// wait for subscribers
		time.Sleep(3 * time.Second)
		manager.logBroadcaster.logC <- &types.Log{
			ID:         "Rei",
			Name:       "nerv",
			Type:       "stdout",
			EntryPoint: "eva0",
			Data:       "data0",
		}
		manager.logBroadcaster.logC <- &types.Log{
			ID:         "Rei",
			Name:       "nerv",
			Type:       "stdout",
			EntryPoint: "eva0",
			Data:       "data1",
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go manager.logBroadcaster.run(ctx)

	// wait for http server to start
	time.Sleep(time.Second)

	reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", "http://127.0.0.1:12310/log/?app=nerv", nil)
	assert.Nil(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(t, err)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for i := 0; i < 2; i++ {
		line, err := reader.ReadBytes('\n')
		assert.Nil(t, err)
		t.Log(string(line))
	}

	logCancel()
	// wait log subscriber to be removed
	time.Sleep(time.Second)

	manager.logBroadcaster.logC <- &types.Log{
		ID:         "Rei",
		Name:       "nerv",
		Type:       "stdout",
		EntryPoint: "eva0",
		Data:       "data1",
	}
	count := 0
	manager.logBroadcaster.subscribersMap.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	assert.Equal(t, count, 0)
}
