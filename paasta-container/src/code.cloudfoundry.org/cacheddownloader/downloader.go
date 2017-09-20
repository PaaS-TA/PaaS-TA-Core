package cacheddownloader

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cloudfoundry/systemcerts"
)

const (
	MAX_DOWNLOAD_ATTEMPTS = 3
	NoBytesReceived       = -1
)

type DownloadCancelledError struct {
	source   string
	duration time.Duration
	written  int64
}

func NewDownloadCancelledError(source string, duration time.Duration, written int64) error {
	return &DownloadCancelledError{
		source:   source,
		duration: duration,
		written:  written,
	}
}

func (e *DownloadCancelledError) Error() string {
	msg := fmt.Sprintf("Download cancelled: source '%s', duration '%s'", e.source, e.duration)
	if e.written != NoBytesReceived {
		msg = fmt.Sprintf("%s, bytes '%d'", msg, e.written)
	}
	return msg
}

type idleTimeoutConn struct {
	Timeout time.Duration
	net.Conn
}

func (c *idleTimeoutConn) Read(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Read(b)
}

func (c *idleTimeoutConn) Write(b []byte) (n int, err error) {
	if err = c.Conn.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return
	}
	return c.Conn.Write(b)
}

type Downloader struct {
	client                    *http.Client
	concurrentDownloadBarrier chan struct{}
}

func NewDownloader(requestTimeout time.Duration, maxConcurrentDownloads int, skipSSLVerification bool, caCertPool *systemcerts.CertPool) *Downloader {
	return NewDownloaderWithIdleTimeout(requestTimeout, 10*time.Second, maxConcurrentDownloads, skipSSLVerification, caCertPool)
}

func NewDownloaderWithIdleTimeout(requestTimeout time.Duration, idleTimeout time.Duration, maxConcurrentDownloads int, skipSSLVerification bool, caCertPool *systemcerts.CertPool) *Downloader {
	var certPool *x509.CertPool
	if caCertPool != nil {
		certPool = caCertPool.AsX509CertPool()
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, 10*time.Second)
			if err != nil {
				return nil, err
			}
			if tc, ok := c.(*net.TCPConn); ok {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(30 * time.Second)
			}
			return &idleTimeoutConn{idleTimeout, c}, nil
		},
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs:            certPool,
			InsecureSkipVerify: skipSSLVerification,
			MinVersion:         tls.VersionTLS10,
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}

	return &Downloader{
		client: client,
		concurrentDownloadBarrier: make(chan struct{}, maxConcurrentDownloads),
	}
}

func (downloader *Downloader) Download(
	url *url.URL,
	createDestination func() (*os.File, error),
	cachingInfoIn CachingInfoType,
	checksum ChecksumInfoType,
	cancelChan <-chan struct{},
) (path string, cachingInfoOut CachingInfoType, err error) {

	startTime := time.Now()

	select {
	case downloader.concurrentDownloadBarrier <- struct{}{}:
	case <-cancelChan:
		return "", CachingInfoType{}, NewDownloadCancelledError("download-barrier", time.Now().Sub(startTime), NoBytesReceived)
	}

	defer func() {
		<-downloader.concurrentDownloadBarrier
	}()

	for attempt := 0; attempt < MAX_DOWNLOAD_ATTEMPTS; attempt++ {
		path, cachingInfoOut, err = downloader.fetchToFile(url, createDestination, cachingInfoIn, checksum, cancelChan)

		if err == nil {
			break
		}

		if _, ok := err.(*DownloadCancelledError); ok {
			break
		}

		if _, ok := err.(*ChecksumFailedError); ok {
			break
		}
	}

	if err != nil {
		return "", CachingInfoType{}, err
	}

	return
}

func (downloader *Downloader) fetchToFile(
	url *url.URL,
	createDestination func() (*os.File, error),
	cachingInfoIn CachingInfoType,
	checksum ChecksumInfoType,
	cancelChan <-chan struct{},
) (string, CachingInfoType, error) {
	var req *http.Request
	var err error

	req, err = http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return "", CachingInfoType{}, err
	}

	if cachingInfoIn.ETag != "" {
		req.Header.Add("If-None-Match", cachingInfoIn.ETag)
	}
	if cachingInfoIn.LastModified != "" {
		req.Header.Add("If-Modified-Since", cachingInfoIn.LastModified)
	}

	completeChan := make(chan struct{})
	defer close(completeChan)

	if transport, ok := downloader.client.Transport.(*http.Transport); ok {
		go func() {
			select {
			case <-completeChan:
			case <-cancelChan:
				transport.CancelRequest(req)
			}
		}()
	}

	startTime := time.Now()

	var resp *http.Response
	resp, err = downloader.client.Do(req)
	if err != nil {
		select {
		case <-cancelChan:
			err = NewDownloadCancelledError("fetch-request", time.Now().Sub(startTime), NoBytesReceived)
		default:
		}
		return "", CachingInfoType{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return "", CachingInfoType{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", CachingInfoType{}, fmt.Errorf("Download failed: Status code %d", resp.StatusCode)
	}

	var destinationFile *os.File
	destinationFile, err = createDestination()
	if err != nil {
		return "", CachingInfoType{}, err
	}

	defer func() {
		destinationFile.Close()
		if err != nil {
			os.Remove(destinationFile.Name())
		}
	}()

	_, err = destinationFile.Seek(0, 0)
	if err != nil {
		return "", CachingInfoType{}, err
	}

	err = destinationFile.Truncate(0)
	if err != nil {
		return "", CachingInfoType{}, err
	}

	go func() {
		select {
		case <-completeChan:
		case <-cancelChan:
			resp.Body.Close()
		}
	}()

	ioWriters := []io.Writer{destinationFile}

	var checksumValidator *hashValidator

	// if checksum data is provided, create the checksum validator
	if checksum.Algorithm != "" || checksum.Value != "" {
		checksumValidator, err = NewHashValidator(checksum.Algorithm)
		if err != nil {
			return "", CachingInfoType{}, err
		}
		ioWriters = append(ioWriters, checksumValidator.hash)
	}

	startTime = time.Now()
	written, err := io.Copy(io.MultiWriter(ioWriters...), resp.Body)
	if err != nil {
		select {
		case <-cancelChan:
			err = NewDownloadCancelledError("copy-body", time.Now().Sub(startTime), written)
		default:
		}
		return "", CachingInfoType{}, err
	}

	cachingInfoOut := CachingInfoType{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	// validate checksum
	if checksumValidator != nil {
		err = checksumValidator.Validate(checksum.Value)
		if err != nil {
			return "", CachingInfoType{}, err
		}
	}

	return destinationFile.Name(), cachingInfoOut, nil
}
