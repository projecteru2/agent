package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof"

	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"

	"github.com/bmizerany/pat"
)

type Handler struct {
}

// URL /version/
func (h *Handler) version(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JSON{"version": common.ERU_AGENT_VERSION})
}

// URL /profile/
func (h *Handler) profile(w http.ResponseWriter, req *http.Request) {
	r := JSON{}
	for _, p := range pprof.Profiles() {
		r[p.Name()] = p.Count()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r)
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
			log.Error(err)
			return
		}
		log.Infof("%s log attached", app)
		logWatcher := types.LogWatcher{
			ID:  utils.RandStringRunes(8),
			App: app, Conn: conn, Buf: buf,
		}
	}
}

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
	log.Infof("http api started %s", addr)
	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Panicf("http api failed %s", err)
		}
	}()
}
