package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/fakes"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/monit_agent/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MonitHandler", func() {
	Describe("ServeHTTP", func() {
		var (
			monitWrapper *fakes.MonitWrapper
			removeStore  *fakes.RemoveStore
			logger       *fakes.Logger

			monitHandler handlers.MonitHandler
		)

		BeforeEach(func() {
			monitWrapper = &fakes.MonitWrapper{}
			removeStore = &fakes.RemoveStore{}
			logger = &fakes.Logger{}

			monitHandler = handlers.NewMonitHandler("start", monitWrapper, removeStore, logger)
		})

		It("calls monit", func() {
			request, err := http.NewRequest("GET", "/start?job=fake_job", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			monitHandler.ServeHTTP(recorder, request)

			Expect(logger.PrintlnCall.Receives.Values).To(Equal([]interface{}{
				"[INFO] monit start fake_job",
			}))
			Expect(monitWrapper.RunCall.CallCount).To(Equal(1))
			Expect(monitWrapper.RunCall.Receives.Args).To(Equal([]string{
				"start",
				"fake_job",
			}))

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		Context("when delete_store is not provided", func() {
			It("deletes the data dir after calling monit", func() {
				request, err := http.NewRequest("GET", "/stop?job=fake_job", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())

				recorder := httptest.NewRecorder()

				monitHandler.ServeHTTP(recorder, request)

				Expect(removeStore.DeleteContentsCall.CallCount).To(Equal(0))

				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})

		Context("when delete_store is provided", func() {
			It("deletes the data dir after calling monit", func() {
				request, err := http.NewRequest("GET", "/stop?job=fake_job&delete_store=true", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())

				recorder := httptest.NewRecorder()

				monitHandler.ServeHTTP(recorder, request)

				Expect(logger.PrintlnCall.Receives.Values).To(Equal([]interface{}{
					"[INFO] deleting /var/vcap/store/fake_job contents",
				}))
				Expect(removeStore.DeleteContentsCall.CallCount).To(Equal(1))
				Expect(removeStore.DeleteContentsCall.Receives.StoreDir).To(Equal("/var/vcap/store/fake_job"))

				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})

		Context("failure cases", func() {
			It("returns an error when monit wrapper fails", func() {
				monitWrapper.RunCall.Returns.Error = errors.New("failed to call monit")
				request, err := http.NewRequest("GET", "/start?job=fake_job", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())
				recorder := httptest.NewRecorder()

				monitHandler.ServeHTTP(recorder, request)
				Expect(logger.PrintlnCall.Receives.Values).To(Equal([]interface{}{
					"[ERR] failed to call monit",
				}))
			})

			It("returns an error when remove store fails", func() {
				removeStore.DeleteContentsCall.Returns.Error = errors.New("failed to delete store")
				request, err := http.NewRequest("GET", "/stop?job=fake_job&delete_store=true", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())
				recorder := httptest.NewRecorder()

				monitHandler.ServeHTTP(recorder, request)
				Expect(logger.PrintlnCall.Receives.Values).To(Equal([]interface{}{
					"[ERR] failed to delete store",
				}))
			})
		})
	})
})
