package handlers

import (
	"mime/multipart"
	"net/http"

	"github.com/cloudfoundry/gosteno"
)

type HttpHandler struct {
	Messages <-chan []byte
	logger   *gosteno.Logger
}

func NewHttpHandler(m <-chan []byte, logger *gosteno.Logger) *HttpHandler {
	return &HttpHandler{Messages: m, logger: logger}
}

func (h *HttpHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	h.logger.Debugf("http handler: ServeHTTP entered with request %v", r.URL)
	defer h.logger.Debugf("http handler: ServeHTTP exited")

	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for message := range h.Messages {
		partWriter, err := mp.CreatePart(nil)
		if err != nil {
			h.logger.Infof("http handler: Client went away while serving recent logs")
			return
		}

		partWriter.Write(message)
	}
}
