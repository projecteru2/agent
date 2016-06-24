package api

import (
	"encoding/json"
	"net/http"
	"runtime/pprof"

	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/store"
	"gitlab.ricebook.net/platform/agent/types"

	"github.com/bmizerany/pat"
)

type Handler struct {
	store store.Store
}

// URL /version/
func (h *Handler) version(req *Request) (int, interface{}) {
	return http.StatusOK, JSON{"version": common.ERU_AGENT_VERSION}
}

// URL /profile/
func (h *Handler) profile(req *Request) (int, interface{}) {
	r := JSON{}
	for _, p := range pprof.Profiles() {
		r[p.Name()] = p.Count()
	}
	return http.StatusOK, r
}

// URL /api/container/add/
func (h *Handler) addNewContainer(req *Request) (int, interface{}) {
	container := &types.Container{}

	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(container); err != nil {
		return http.StatusBadRequest, JSON{"message": "wrong JSON format"}
	}

	if err := h.store.UpdateContainer(container); err != nil {
		return http.StatusBadRequest, JSON{"message": err}
	}
	return http.StatusOK, JSON{"message": "ok"}
}

func Serve(addr string, store store.Store) {
	if addr == "" {
		return
	}

	h := &Handler{store}
	restfulAPIServer := pat.New()
	handlers := map[string]map[string]func(*Request) (int, interface{}){
		"GET": {
			"/profile/": h.profile,
			"/version/": h.version,
		},
		"POST": {
			"/api/container/add/": h.addNewContainer,
		},
	}

	for method, routes := range handlers {
		for route, handler := range routes {
			restfulAPIServer.Add(method, route, http.HandlerFunc(JSONWrapper(handler)))
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
