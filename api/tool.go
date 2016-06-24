package api

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type JSON map[string]interface{}

func JSONWrapper(f func(*Request) (int, interface{})) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		r := NewRequest(req)
		w.Header().Set("Content-Type", "application/json")
		code, result := f(r)
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(result)
	}
}

func Atoi(s string, def int) int {
	if r, err := strconv.Atoi(s); err != nil {
		return def
	} else {
		return r
	}
}
