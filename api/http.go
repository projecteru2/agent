package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof" //nolint:nolintlint
	"time"

	_ "net/http/pprof" //nolint

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/manager/workload"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"
)

type JSON map[string]interface{}

type Handler struct {
	config           *types.Config
	workloadsManager *workload.Manager
}

func NewHandler(config *types.Config, workloadsManager *workload.Manager) *Handler {
	return &Handler{
		config:           config,
		workloadsManager: workloadsManager,
	}
}

func (h *Handler) Serve() {
	if h.config.API.Addr == "" {
		return
	}
	logger := log.WithFunc("serve")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /profile/{$}", h.profile)
	mux.HandleFunc("GET /version/{$}", h.version)
	mux.HandleFunc("GET /log/{$}", h.log)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	logger.Infof(nil, "http api started %s", h.config.API.Addr) //nolint

	server := &http.Server{
		Addr:              h.config.API.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		logger.Error(nil, err, "http api start failed") //nolint
	}
}

func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(JSON{"version": version.VERSION})
}

func (h *Handler) profile(w http.ResponseWriter, _ *http.Request) {
	r := JSON{}
	for _, p := range pprof.Profiles() {
		r[p.Name()] = p.Count()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(r)
}

func (h *Handler) log(w http.ResponseWriter, req *http.Request) {
	app := req.URL.Query().Get("app")
	if app == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	logger := log.WithFunc("log").WithField("path", "/log")
	// the status line must go out before the hijack, otherwise clients see no response
	w.WriteHeader(http.StatusOK)
	if hijack, ok := w.(http.Hijacker); ok {
		conn, buf, err := hijack.Hijack()
		if err != nil {
			logger.Error(req.Context(), err, "connect failed")
			return
		}
		defer conn.Close()
		h.workloadsManager.PullLog(req.Context(), app, buf)
	}
}
