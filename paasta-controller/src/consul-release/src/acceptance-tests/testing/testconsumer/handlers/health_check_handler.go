package handlers

import (
	"io/ioutil"
	"net/http"
)

type HealthCheckHandler struct {
	ok bool
}

func NewHealthCheckHandler() HealthCheckHandler {
	return HealthCheckHandler{
		ok: true,
	}
}

func (c *HealthCheckHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		if !c.ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		return
	case "POST":
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		c.ok = string(body) == "true"
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
