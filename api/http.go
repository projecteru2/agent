package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof"
	"strings"

	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
	"gitlab.ricebook.net/platform/agent/watcher"

	"github.com/bmizerany/pat"
)

type JSON map[string]interface{}

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
		logConsumer := &types.LogConsumer{
			ID:  utils.RandStringRunes(8),
			App: app, Conn: conn, Buf: buf,
		}
		watcher.LogMonitor.ConsumerC <- logConsumer
		log.Infof("%s %s log attached", app, logConsumer.ID)
	}
}

// URL /setvip/
func (h *Handler) setVip(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vip := req.FormValue("vip")
	if vip == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	inter := req.FormValue("interface")
	if inter == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := utils.SetVip(vip, inter)
	if err != nil {
		log.Errorf("Error occurs while setting vip: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

// URL /delvip/
func (h *Handler) delVip(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	vip := req.FormValue("vip")
	if vip == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	inter := req.FormValue("interface")
	if inter == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err := utils.DelVip(vip, inter)
	if err != nil {
		log.Errorf("Error occurs while deleting vip: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	return

}

// URL /checkvip/
func (h *Handler) checkVip(w http.ResponseWriter, req *http.Request) {
	r := JSON{}
	w.Header().Set("Content-Type", "application/json")
	vip := req.URL.Query().Get("vip")
	if vip == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	inter := req.URL.Query().Get("interface")
	if inter == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	msg, err := utils.CheckVip(inter)
	if err != nil {
		log.Errorf("Error occurs while checking vip: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r["status"] = "VipNotExists"
	if strings.Contains(msg, vip) {
		r["status"] = "VipExists"
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r)
}

func Serve(addr string) {
	if addr == "" {
		return
	}

	h := &Handler{}
	restfulAPIServer := pat.New()
	handlers := map[string]map[string]func(http.ResponseWriter, *http.Request){
		"GET": {
			"/profile/":  h.profile,
			"/version/":  h.version,
			"/log/":      h.log,
			"/checkvip/": h.checkVip,
		},
		"POST": {
			"/setvip/": h.setVip,
			"/delvip/": h.delVip,
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
