package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof"

	// enable profile
	_ "net/http/pprof" // nolint

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"
	"github.com/projecteru2/agent/watcher"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/bmizerany/pat"
)

// JSON define a json
type JSON map[string]interface{}

// Handler define handler
type Handler struct {
}

// URL /version/
func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(JSON{"version": version.VERSION})
}

// URL /profile/
func (h *Handler) profile(w http.ResponseWriter, _ *http.Request) {
	r := JSON{}
	for _, p := range pprof.Profiles() {
		r[p.Name()] = p.Count()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(r)
}

// URL /log/
func (h *Handler) log(w http.ResponseWriter, req *http.Request) {
	app := req.URL.Query().Get("app")
	if app == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// fuck httpie
	w.WriteHeader(http.StatusOK)
	if hijack, ok := w.(http.Hijacker); ok {
		conn, buf, err := hijack.Hijack()
		if err != nil {
			log.Errorf("[apiLog] connect failed %v", err)
			return
		}
		logConsumer := &types.LogConsumer{
			ID:  coreutils.RandomString(8),
			App: app, Conn: conn, Buf: buf,
		}
		watcher.LogMonitor.ConsumerC <- logConsumer
		log.Infof("[apiLog] %s %s log attached", app, logConsumer.ID)
	}
}

// Serve start a api service
func Serve(addr string) {
	if addr == "" {
		return
	}

	h := &Handler{}
	restfulAPIServer := pat.New()
	handlers := map[string]map[string]func(http.ResponseWriter, *http.Request){
		"GET": {
			"/profile/": h.profile,
			"/version/": h.version,
			"/log/":     h.log,
		},
	}

	for method, routes := range handlers {
		for route, handler := range routes {
			restfulAPIServer.Add(method, route, http.HandlerFunc(handler))
		}
	}

	http.Handle("/", restfulAPIServer)
	http.Handle("/metrics", promhttp.Handler())
	log.Infof("[apiServe] http api started %s", addr)
	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Panicf("http api failed %s", err)
		}
	}()
}
