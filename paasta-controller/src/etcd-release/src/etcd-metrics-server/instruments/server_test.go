package instruments_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/fakes"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instruments"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instrumentation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server Instrumentation", func() {
	var (
		etcdServer *httptest.Server
		server     *instruments.Server
		fakeGetter *fakes.Getter
	)

	BeforeEach(func() {
		fakeGetter = &fakes.Getter{}
	})

	Context("when the metrics fetch succesfully", func() {
		Context("when the etcd server gives valid JSON", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/self":
						if req.Method == "GET" {
							w.Write([]byte(`
								{
									"name": "node1",
									"state": "StateLeader",

									"leaderInfo": {
										"name": "node1",
										"uptime": "forever"
									},

									"recvAppendRequestCnt": 1234,
									"recvPkgRate": 5678.0,
									"recvBandwidthRate": 9101112.13,

									"sendAppendRequestCnt": 4321,
									"sendPkgRate": 8765.0,
									"sendBandwidthRate": 1211109.8
								}
							`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				server = instruments.NewServer(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("should return them", func() {
				context := server.Emit()

				Expect(fakeGetter.GetCall.CallCount).To(Equal(1))
				Expect(context.Name).Should(Equal("server"))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "IsLeader",
					Value: 1,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "SendingBandwidthRate",
					Value: 1211109.8,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "ReceivingBandwidthRate",
					Value: 9101112.13,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "SendingRequestRate",
					Value: 8765.0,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "ReceivingRequestRate",
					Value: 5678.0,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "SentAppendRequests",
					Value: uint64(4321),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "ReceivedAppendRequests",
					Value: uint64(1234),
				}))
			})
		})

		Context("when the etcd server gives invalid JSON", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/self":
						if req.Method == "GET" {
							w.Write([]byte("ß"))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))
				server = instruments.NewServer(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := server.Emit()
				Expect(context.Metrics).Should(BeEmpty())
			})
		})
	})

	Context("when the metrics fail to fetch", func() {
		BeforeEach(func() {
			etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
			server = instruments.NewServer(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
		})

		It("should not return them", func() {
			context := server.Emit()
			Expect(context.Metrics).Should(BeEmpty())
		})
	})
})
