package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/fakes"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MonitJobStatusHandler", func() {
	Describe("ServeHTTP", func() {
		var (
			monitWrapper *fakes.MonitWrapper
			logger       *fakes.Logger

			monitJobStatusHandler handlers.MonitJobStatusHandler
		)

		BeforeEach(func() {
			monitWrapper = &fakes.MonitWrapper{}
			logger = &fakes.Logger{}

			monitWrapper.OutputCall.Returns.Output = `The Monit daemon 5.2.5 uptime: 19m

Process 'some_job'                  running
Process 'fake_job'                  not monitored
Process 'other_job'                 running
`

			monitJobStatusHandler = handlers.NewMonitJobStatusHandler(monitWrapper, logger)
		})

		It("calls monit summary and gets job status", func() {
			request, err := http.NewRequest("GET", "/job_status?job=fake_job", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			monitJobStatusHandler.ServeHTTP(recorder, request)

			Expect(logger.PrintlnCall.AllValues).To(ContainElement([]interface{}{
				"[INFO] get job status of fake_job using monit summary",
			}))
			Expect(monitWrapper.OutputCall.CallCount).To(Equal(1))
			Expect(monitWrapper.OutputCall.Receives.Args).To(Equal([]string{
				"summary",
			}))
			Expect(logger.PrintlnCall.AllValues).To(ContainElement([]interface{}{
				fmt.Sprintf("[INFO] monit summary output: %s", monitWrapper.OutputCall.Returns.Output),
			}))

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Body.String()).To(Equal("not monitored"))
		})

		Context("failure cases", func() {
			It("returns an error when monit wrapper fails", func() {
				monitWrapper.OutputCall.Returns.Error = errors.New("failed to call monit")
				request, err := http.NewRequest("GET", "/start?job=fake_job", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())
				recorder := httptest.NewRecorder()

				monitJobStatusHandler.ServeHTTP(recorder, request)
				Expect(logger.PrintlnCall.AllValues).To(ContainElement([]interface{}{
					"[ERR] failed to call monit",
				}))
			})
		})
	})
})
