package instruments_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/fakes"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instrumentation"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instruments"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Leader Instrumentation", func() {
	var (
		leader     *instruments.Leader
		etcdServer *httptest.Server
		fakeGetter *fakes.Getter
	)

	BeforeEach(func() {
		fakeGetter = &fakes.Getter{}
	})

	Context("when the metrics fetch succesfully", func() {
		Context("when the etcd server is a leader", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/leader":
						if req.Method == "GET" {
							w.Write([]byte(`
								{
								  "followers": {
									"node1": {
									  "counts": {
										"success": 277031,
										"fail": 0
									  },
									  "latency": {
										"maximum": 65.038854,
										"minimum": 0.124347,
										"standardDeviation": 0.41350537505117785,
										"average": 0.37073788356538245,
										"current": 1.0
									  }
									},
									"node2": {
									  "counts": {
										"success": 277031,
										"fail": 0
									  },
									  "latency": {
										"maximum": 65.038854,
										"minimum": 0.124347,
										"standardDeviation": 0.41350537505117785,
										"average": 0.37073788356538245,
										"current": 2.0
									  }
									}
								  },
								  "leader": "node0"
								}
							`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				leader = instruments.NewLeader(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("should return them", func() {
				context := leader.Emit()

				Expect(context.Name).Should(Equal("leader"))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "Followers",
					Value: 2,
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "Latency",
					Value: 1.0,
					Tags: map[string]interface{}{
						"follower": "node1",
					},
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "Latency",
					Value: 2.0,
					Tags: map[string]interface{}{
						"follower": "node2",
					},
				}))

				Expect(fakeGetter.GetCall.CallCount).To(Equal(1))
			})
		})

		Context("when the etcd server is a follower", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/leader":
						if req.Method == "GET" {
							w.Header().Set("Location", "http://some/other/leader")
							w.WriteHeader(http.StatusFound)
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				leader = instruments.NewLeader(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := leader.Emit()
				Expect(context.Metrics).ShouldNot(BeNil())
				Expect(context.Metrics).Should(BeEmpty())
			})
		})

		Context("when the etcd server gives invalid JSON", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/leader":
						if req.Method == "GET" {
							w.Write([]byte("ß"))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				leader = instruments.NewLeader(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := leader.Emit()
				Expect(context.Metrics).Should(BeEmpty())
			})
		})

		Context("when the request to the etcd server times out", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/leader":
						if req.Method == "GET" {
							w.Write([]byte(`{"followers": {}, "leader": "node0" }`))
							time.Sleep(1100 * time.Millisecond)
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				leader = instruments.NewLeader(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := leader.Emit()
				Expect(context.Metrics).Should(BeEmpty())
			})
		})
	})

	Context("when the metrics fail to fetch", func() {
		BeforeEach(func() {
			etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))
			leader = instruments.NewLeader(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
		})

		It("should not return them", func() {
			context := leader.Emit()
			Expect(context.Metrics).Should(BeEmpty())
		})
	})
})
