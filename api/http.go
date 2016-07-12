package api

import (
	"net/http"
	"runtime/pprof"

	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/common"

	"github.com/bmizerany/pat"
)

type Handler struct {
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

func Serve(addr string) {
	if addr == "" {
		return
	}

	h := &Handler{}
	restfulAPIServer := pat.New()
	handlers := map[string]map[string]func(*Request) (int, interface{}){
		"GET": {
			"/profile/": h.profile,
			"/version/": h.version,
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
