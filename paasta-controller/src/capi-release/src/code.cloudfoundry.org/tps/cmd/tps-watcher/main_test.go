package main_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

const watcherLockName = "tps_watcher_lock"

var _ = Describe("TPS", func() {
	startWatcher := func(check bool) (ifrit.Process, *ginkgomon.Runner) {
		if !check {
			runner.StartCheck = ""
		}

		return ginkgomon.Invoke(runner), runner
	}

	var (
		domain string
	)

	BeforeEach(func() {
		domain = cc_messages.AppLRPDomain
	})

	AfterEach(func() {
		if watcher != nil {
			watcher.Signal(os.Kill)
			Eventually(watcher.Wait()).Should(Receive())
		}
	})

	Describe("Crashed Apps", func() {
		var (
			ready chan struct{}
		)

		BeforeEach(func() {
			ready = make(chan struct{})
			fakeCC.RouteToHandler("POST", "/internal/apps/some-process-guid/crashed", func(res http.ResponseWriter, req *http.Request) {
				var appCrashed cc_messages.AppCrashedRequest

				bytes, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				req.Body.Close()

				err = json.Unmarshal(bytes, &appCrashed)
				Expect(err).NotTo(HaveOccurred())

				Expect(appCrashed.CrashTimestamp).NotTo(BeZero())
				appCrashed.CrashTimestamp = 0

				Expect(appCrashed).To(Equal(cc_messages.AppCrashedRequest{
					Instance:        "some-instance-guid-1",
					Index:           1,
					Reason:          "CRASHED",
					ExitDescription: "out of memory",
					CrashCount:      1,
				}))

				close(ready)
			})

			lrpKey := models.NewActualLRPKey("some-process-guid", 1, domain)
			instanceKey := models.NewActualLRPInstanceKey("some-instance-guid-1", "cell-id")
			netInfo := models.NewActualLRPNetInfo("1.2.3.4", models.NewPortMapping(65100, 8080))
			beforeActualLRP := *models.NewRunningActualLRP(lrpKey, instanceKey, netInfo, 0)
			afterActualLRP := beforeActualLRP
			afterActualLRP.State = models.ActualLRPStateCrashed
			afterActualLRP.Since = int64(1)
			afterActualLRP.CrashCount = 1
			afterActualLRP.CrashReason = "out of memory"

			fakeBBS.RouteToHandler("GET", "/v1/events",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					flusher := w.(http.Flusher)
					flusher.Flush()
					closeNotifier := w.(http.CloseNotifier).CloseNotify()
					event := models.NewActualLRPCrashedEvent(&afterActualLRP)

					sseEvent, err := events.NewEventFromModelEvent(0, event)
					Expect(err).NotTo(HaveOccurred())

					err = sseEvent.Write(w)
					Expect(err).NotTo(HaveOccurred())

					flusher.Flush()

					<-closeNotifier
				},
			)
		})

		JustBeforeEach(func() {
			watcher, _ = startWatcher(true)
		})

		It("POSTs to the CC that the application has crashed", func() {
			Eventually(ready, 5*time.Second).Should(BeClosed())
		})
	})

	Context("when the watcher loses the lock", func() {
		BeforeEach(func() {
			fakeBBS.RouteToHandler("GET", "/v1/events",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					closeNotifier := w.(http.CloseNotifier).CloseNotify()

					<-closeNotifier
				},
			)

			watcher, _ = startWatcher(true)
		})

		JustBeforeEach(func() {
			consulRunner.Reset()
		})

		AfterEach(func() {
			ginkgomon.Interrupt(watcher, 5)
		})

		It("exits with an error", func() {
			Eventually(watcher.Wait(), 5).Should(Receive(HaveOccurred()))
		})
	})

	Context("when the watcher initially does not have the lock", func() {
		var runner *ginkgomon.Runner
		var competingWatcherProcess ifrit.Process

		BeforeEach(func() {
			fakeBBS.RouteToHandler("GET", "/v1/events",
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
					w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Add("Connection", "keep-alive")

					w.WriteHeader(http.StatusOK)

					closeNotifier := w.(http.CloseNotifier).CloseNotify()

					<-closeNotifier
				},
			)

			competingWatcher := locket.NewLock(logger, consulRunner.NewClient(), locket.LockSchemaPath(watcherLockName), []byte("something-else"), clock.NewClock(), locket.RetryInterval, locket.LockTTL)
			competingWatcherProcess = ifrit.Invoke(competingWatcher)
		})

		JustBeforeEach(func() {
			watcher, runner = startWatcher(false)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(watcher, 5)
			ginkgomon.Kill(competingWatcherProcess)
		})

		It("does not start", func() {
			Consistently(runner.Buffer, 5*time.Second).ShouldNot(gbytes.Say("tps-watcher.started"))
		})

		Context("when the lock becomes available", func() {
			BeforeEach(func() {
				ginkgomon.Kill(competingWatcherProcess)
			})

			It("is updated", func() {
				Eventually(runner.Buffer, 5*time.Second).Should(gbytes.Say("tps-watcher.started"))
			})
		})
	})
})
