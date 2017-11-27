package logspammer_test

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/logspammer"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	noaaerrors "github.com/cloudfoundry/noaa/errors"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeNoaaConsumer struct {
	StreamCall struct {
		CallCount int
		Receives  struct {
			AppGuid   string
			AuthToken string
		}
		Returns struct {
			OutputChan chan *events.Envelope
			ErrChan    chan error
		}
	}
}

func (f *fakeNoaaConsumer) Stream(appGuid string, authToken string) (outputChan <-chan *events.Envelope, errorChan <-chan error) {
	f.StreamCall.CallCount++
	f.StreamCall.Receives.AppGuid = appGuid
	f.StreamCall.Receives.AuthToken = authToken

	return f.StreamCall.Returns.OutputChan, f.StreamCall.Returns.ErrChan
}

var _ = Describe("logspammer", func() {
	var (
		appServer          *httptest.Server
		spammer            *logspammer.Spammer
		noaaConsumer       *fakeNoaaConsumer
		appServerCallCount int32
		skipStream         bool
		l                  sync.Mutex
		channelLock        sync.Mutex
	)

	var setSkipStream = func(b bool) {
		l.Lock()
		defer l.Unlock()
		skipStream = b
	}

	var getSkipStream = func() bool {
		l.Lock()
		defer l.Unlock()
		return skipStream
	}

	var writeLogToChannel = func(envelope *events.Envelope) {
		channelLock.Lock()
		defer channelLock.Unlock()

		noaaConsumer.StreamCall.Returns.OutputChan <- envelope
	}

	var writeErrorToErrorChannel = func(err error) {
		channelLock.Lock()
		defer channelLock.Unlock()

		noaaConsumer.StreamCall.Returns.ErrChan <- err
	}

	BeforeEach(func() {
		atomic.StoreInt32(&appServerCallCount, 0)
		noaaConsumer = &fakeNoaaConsumer{}
		noaaConsumer.StreamCall.Returns.OutputChan = make(chan *events.Envelope)
		noaaConsumer.StreamCall.Returns.ErrChan = make(chan error)

		appServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/log") && req.Method == "GET" {
				parts := strings.Split(req.URL.Path, "/")
				w.WriteHeader(http.StatusOK)
				if getSkipStream() == false {
					sourceType := "APP/PROC/WEB"
					messageType := events.LogMessage_OUT

					envelope := &events.Envelope{
						LogMessage: &events.LogMessage{
							MessageType: &messageType,
							SourceType:  &sourceType,
							Message:     []byte(parts[2]),
						},
					}
					writeLogToChannel(envelope)
				}
				atomic.AddInt32(&appServerCallCount, 1)
				return
			}
			w.WriteHeader(http.StatusTeapot)
		}))

		fakeStreamGenerator := func() (<-chan *events.Envelope, <-chan error) {
			return noaaConsumer.Stream("some-guid", "some-token")
		}

		spammer = logspammer.NewSpammer(ioutil.Discard, appServer.URL, fakeStreamGenerator, 10*time.Millisecond)
		err := spammer.Start()
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() int {
			return len(spammer.LogMessages())
		}).Should(BeNumerically(">", 0))
	})

	AfterEach(func() {
		logspammer.ResetTimeNow()
	})

	Describe("Check", func() {
		It("check for log interruptions", func() {
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			spammerErr, logMissingErr := spammer.Check()
			Expect(spammerErr).NotTo(HaveOccurred())
			Expect(logMissingErr).NotTo(HaveOccurred())

			Expect(int(atomic.LoadInt32(&appServerCallCount))).To(Equal(len(spammer.LogMessages())))
		})

		Context("failure cases", func() {
			It("returns an error when the spammer fails to write to the app", func() {
				fakeStreamGenerator := func() (<-chan *events.Envelope, <-chan error) {
					return noaaConsumer.Stream("some-guid", "some-token")
				}

				spammer.Stop()
				spammer = logspammer.NewSpammer(ioutil.Discard, "", fakeStreamGenerator, 0)
				err := spammer.Start()
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(100 * time.Millisecond)

				err = spammer.Stop()
				Expect(err).NotTo(HaveOccurred())

				spammerErr, logMissingErr := spammer.Check()
				Expect(spammerErr).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				Expect(logMissingErr).NotTo(HaveOccurred())
			})

			It("returns an error when an app log line is missing", func() {
				var testUnixSeconds, timeCallCount int64
				// Hard-code the time for the purposes of matching log entries below.
				testUnixSeconds = 1515151515
				logspammer.SetTimeNow(func() time.Time {
					// Increment the nanoseconds every time the code requests the current time.
					timeCallCount++
					return time.Unix(testUnixSeconds, timeCallCount)
				})

				setSkipStream(true)
				logCount := len(spammer.LogMessages())

				Eventually(func() int {
					return int(atomic.LoadInt32(&appServerCallCount))
				}, "1m", "10ms").Should(BeNumerically(">", logCount+10))

				setSkipStream(false)

				err := spammer.Stop()
				Expect(err).NotTo(HaveOccurred())

				spammerErrs, missingLogErrs := spammer.Check()
				Expect(spammerErrs).NotTo(HaveOccurred())
				// We are constructing the expected timestamps of the "missing"
				// log entries to account for different timezones when running
				// the tests.
				expectedErrorSet := helpers.ErrorSet{}

				var i int64
				// There should be exactly 11 missing log entries based on the
				// "Eventually" step above.
				for i = 1; i <= 11; i++ {
					expectedErrorSet[fmt.Sprintf("missing log entry: %s", time.Unix(testUnixSeconds, i).Format(logspammer.TimeFormat))] = 1
				}

				Expect(missingLogErrs).To(Equal(expectedErrorSet))
			})
		})
	})

	Describe("Start", func() {
		It("ignores non source_type APP messages", func() {
			sourceType := "not-app-source-type"
			messageType := events.LogMessage_OUT

			envelope := &events.Envelope{
				LogMessage: &events.LogMessage{
					MessageType: &messageType,
					SourceType:  &sourceType,
					Message:     []byte("NOT-AN-APP-MESSAGE"),
				},
			}
			writeLogToChannel(envelope)

			time.Sleep(5 * time.Millisecond)
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			messages := spammer.LogMessages()

			Expect(messages).ToNot(HaveLen(0))

			for _, message := range messages {
				Expect(message).ToNot(ContainSubstring("NOT-AN-APP-MESSAGE"))
			}

		})

		It("ignores nil log message", func() {
			sourceType := "APP/PROC/WEB"
			messageType := events.LogMessage_OUT
			envelopeNilMessage := &events.Envelope{
				LogMessage: nil,
			}

			envelope := &events.Envelope{
				LogMessage: &events.LogMessage{
					MessageType: &messageType,
					SourceType:  &sourceType,
					Message:     []byte("Message written after nil message"),
				},
			}

			writeLogToChannel(envelopeNilMessage)
			writeLogToChannel(envelope)

			Eventually(func() bool {
				messages := spammer.LogMessages()
				for _, message := range messages {
					if strings.Contains(message, "Message written after nil message") {
						return true
					}
				}

				return false
			}).Should(BeTrue())

			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())
		})

		It("ignores non message_type OUT messages", func() {
			sourceType := "APP/PROC/WEB"
			messageType := events.LogMessage_ERR

			envelope := &events.Envelope{
				LogMessage: &events.LogMessage{
					MessageType: &messageType,
					SourceType:  &sourceType,
					Message:     []byte("NOT-AN-OUT-MESSAGE"),
				},
			}
			writeLogToChannel(envelope)

			time.Sleep(5 * time.Millisecond)
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			messages := spammer.LogMessages()

			Expect(messages).ToNot(HaveLen(0))

			for _, message := range messages {
				Expect(message).ToNot(ContainSubstring("NOT-AN-OUT-MESSAGE"))
			}
		})

		It("refreshes when noaa gets unauthorised error message", func() {
			writeErrorToErrorChannel(noaaerrors.NewUnauthorizedError("token expired"))

			time.Sleep(5 * time.Millisecond)
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			Expect(noaaConsumer.StreamCall.CallCount).To(Equal(2))
		})

		It("ignore when noaa gets other errors on channel", func() {
			writeErrorToErrorChannel(errors.New("unknown"))

			time.Sleep(5 * time.Millisecond)
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			Expect(noaaConsumer.StreamCall.CallCount).To(Equal(1))
		})
	})

	Describe("Stop", func() {
		It("no longer streams messages when the spammer has been stopped", func() {
			err := spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				select {
				case noaaConsumer.StreamCall.Returns.OutputChan <- nil:
					return true
				case <-time.After(10 * time.Millisecond):
					return false
				}
			}, "100ms", "10ms").Should(BeFalse())
		})
	})
})
