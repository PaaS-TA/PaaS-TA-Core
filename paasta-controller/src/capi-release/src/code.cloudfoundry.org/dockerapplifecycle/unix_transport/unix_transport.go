package unix_transport

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func New(socketPath string) *http.Transport {
	unixTransport := &http.Transport{}
	unixTransport.RegisterProtocol("unix", NewUnixRoundTripper(socketPath))
	return unixTransport
}

type UnixRoundTripper struct {
	path string
	conn httputil.ClientConn
}

func NewUnixRoundTripper(path string) *UnixRoundTripper {
	return &UnixRoundTripper{path: path}
}

// The RoundTripper (http://golang.org/pkg/net/http/#RoundTripper) for the socket transport dials the socket
// each time a request is made.
func (roundTripper UnixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	conn, err := net.Dial("unix", roundTripper.path)
	if err != nil {
		return nil, err
	}

	socketClientConn := httputil.NewClientConn(conn, nil)
	defer socketClientConn.Close()

	newReq, err := roundTripper.rewriteRequest(req)
	if err != nil {
		return nil, err
	}

	return socketClientConn.Do(newReq)
}

func (roundTripper *UnixRoundTripper) rewriteRequest(req *http.Request) (*http.Request, error) {
	requestPath := req.URL.Path
	if !strings.HasPrefix(requestPath, roundTripper.path) {
		return nil, fmt.Errorf("Wrong unix socket [unix://%s]. Expected unix socket is [%s]", requestPath, roundTripper.path)
	}

	reqPath := strings.TrimPrefix(requestPath, roundTripper.path)
	newReqUrl := fmt.Sprintf("unix://%s", reqPath)

	var err error
	newURL, err := url.Parse(newReqUrl)
	if err != nil {
		return nil, err
	}

	req.URL.Path = newURL.Path
	req.URL.Host = roundTripper.path
	return req, nil

}
