package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof" // nolint
	// enable profile
	_ "net/http/pprof" // nolint

	"github.com/projecteru2/agent/manager/workload"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"

	"github.com/bmizerany/pat"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// JSON define a json
type JSON map[string]interface{}

// Handler define handler
type Handler struct {
	config          *types.Config
	workloadManager *workload.Manager
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
		defer conn.Close()
		h.workloadManager.Subscribe(req.Context(), app, buf)
	}
}

// NewHandler new api http handler
func NewHandler(config *types.Config, workloadManager *workload.Manager) *Handler {
	return &Handler{
		config:          config,
		workloadManager: workloadManager,
	}
}

// Serve start a api service
// blocks by http.ListenAndServe
// run this in a separated goroutine
func (h *Handler) Serve() {
	if h.config.API.Addr == "" {
		return
	}

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
	log.Infof("[apiServe] http api started %s", h.config.API.Addr)

	err := http.ListenAndServe(h.config.API.Addr, nil)
	if err != nil {
		log.Panicf("http api failed %s", err)
	}
}
