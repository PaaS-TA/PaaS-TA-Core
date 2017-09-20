package ccclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
)

const (
	JOB_QUEUED   = "queued"
	JOB_RUNNING  = "running"
	JOB_FAILED   = "failed"
	JOB_FINISHED = "finished"
)

type poller struct {
	logger lager.Logger

	client       *http.Client
	pollInterval time.Duration
}

func NewPoller(logger lager.Logger, httpClient *http.Client, pollInterval time.Duration) Poller {
	return &poller{
		client:       httpClient,
		pollInterval: pollInterval,
		logger:       logger.Session("poller"),
	}
}

func (p *poller) Poll(fallbackURL *url.URL, res *http.Response, cancelChan <-chan struct{}) error {
	body, err := p.parsePollingResponse(res)
	if err != nil {
		p.logger.Error("failed-parsing-polling-response", err)
		return err
	}

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for i := 0; ; i++ {
		p.logger.Info("checking-cc-job-status", lager.Data{"attempt-number": i, "status": body.Entity.Status})

		switch body.Entity.Status {
		case JOB_QUEUED, JOB_RUNNING:
		case JOB_FINISHED:
			p.logger.Info("cc-job-finished")
			return nil
		case JOB_FAILED:
			err := fmt.Errorf("upload job failed")
			p.logger.Error("cc-job-failed", err)
			return err
		default:
			err := fmt.Errorf("unknown job status: %s", body.Entity.Status)
			p.logger.Error("cc-job-unknown-status", err)
			return err
		}

		select {
		case <-ticker.C:
			pollingUrl, err := url.Parse(body.Metadata.Url)
			if err != nil {
				p.logger.Error("failed-parsing-url", err, lager.Data{"url": body.Metadata.Url})
				return err
			}

			if pollingUrl.Host == "" {
				pollingUrl.Scheme = fallbackURL.Scheme
				pollingUrl.Host = fallbackURL.Host
			}

			req, err := http.NewRequest("GET", pollingUrl.String(), nil)
			if err != nil {
				p.logger.Error("failed-generating-request", err, lager.Data{"url": pollingUrl.String()})
				return err
			}

			completion := make(chan struct{})
			go func() {
				select {
				case <-cancelChan:
					if canceller, ok := p.client.Transport.(requestCanceller); ok {
						canceller.CancelRequest(req)
					} else {
						p.logger.Error("Invalid transport, does not support CancelRequest", nil, lager.Data{"transport": p.client.Transport})
					}
				case <-completion:
				}
			}()

			p.logger.Info("making-request-to-polling-endpoint")
			res, err := p.client.Do(req)
			close(completion)
			if err != nil {
				p.logger.Error("failed-making-request-to-polling-endpoint", err)
				return err
			}
			p.logger.Info("succeeded-making-request-to-polling-endpoint")

			body, err = p.parsePollingResponse(res)
			if err != nil {
				p.logger.Error("failed-parsing-polling-response", err)
				return err
			}
		case <-cancelChan:
			err := fmt.Errorf("upstream request was cancelled")
			p.logger.Error("upstream-request-cancelled", err)
			return err
		}
	}
}

func (p *poller) parsePollingResponse(res *http.Response) (pollingResponseBody, error) {
	body := pollingResponseBody{}
	err := json.NewDecoder(res.Body).Decode(&body)
	res.Body.Close()
	return body, err
}

type pollingResponseBody struct {
	Metadata struct {
		Url string
	}
	Entity struct {
		Status string
	}
}
