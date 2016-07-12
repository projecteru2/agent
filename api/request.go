package api

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/utils"
)

type Request struct {
	http.Request
	Start int
	Limit int
}

func (r *Request) Init() {
	r.Start = utils.Atoi(r.Form.Get("start"), 0)
	r.Limit = utils.Atoi(r.Form.Get("limit"), 20)
}

func NewRequest(r *http.Request) *Request {
	req := &Request{*r, 0, 20}
	req.Init()
	log.Debugf("HTTP request %s %s", req.Method, req.URL.Path)
	return req
}
