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

	handler := func(w http.ResponseWriter, req *http.Request) {
		app := req.URL.Query().Get("app")
		if app == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// fuck httpie
		w.WriteHeader(http.StatusOK)
		if hijack, ok := w.(http.Hijacker); ok {
			conn, buf, err := hijack.Hijack()
			assert.Nil(t, err)
			defer conn.Close()
			manager.PullLog(req.Context(), app, buf)
		}
	}

	go func() {
		restfulAPIServer := pat.New()
		restfulAPIServer.Add("GET", "/log/", http.HandlerFunc(handler))
		http.Handle("/", restfulAPIServer)
		http.ListenAndServe(":12310", nil)
	}()

	go func() {
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

	time.Sleep(2 * time.Second)
	resp, err := http.Get("http://127.0.0.1:12310/log/?app=nerv")
	assert.Nil(t, err)

	reader := bufio.NewReader(resp.Body)
	for i := 0; i < 2; i++ {
		line, err := reader.ReadBytes('\n')
		assert.Nil(t, err)
		t.Log(string(line))
	}
}
