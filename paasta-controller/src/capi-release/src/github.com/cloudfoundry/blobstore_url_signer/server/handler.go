package server

import (
	"io"
	"net/http"
	"net/url"

	"github.com/cloudfoundry/blobstore_url_signer/signer"
)

//go:generate counterfeiter -o fakes/fake_server_handlers.go . ServerHandlers
type ServerHandlers interface {
	SignUrl(w http.ResponseWriter, r *http.Request)
}

type handlers struct {
	signer signer.Signer
}

func NewServerHandlers(signer signer.Signer) ServerHandlers {
	return &handlers{
		signer: signer,
	}
}

func (h *handlers) SignUrl(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(r.URL.String())
	queries, _ := url.ParseQuery(u.RawQuery)
	expirationDate := queries["expires"][0]
	path := queries["path"][0]

	redirectUrl := h.signer.Sign(expirationDate, path)
	io.WriteString(w, redirectUrl)
}
