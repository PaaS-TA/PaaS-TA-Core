package static

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

func New(dir, pathPrefix string, logger lager.Logger) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	stripped := http.StripPrefix(pathPrefix, fileServer)
	return loggingHandler{
		logger:          logger,
		originalHandler: stripped,
	}
}
