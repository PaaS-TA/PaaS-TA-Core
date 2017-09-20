package instruments_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/fakes"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instrumentation"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/instruments"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store Instrumentation", func() {
	var (
		etcdServer *httptest.Server
		store      *instruments.Store
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
					case "/v2/stats/store":
						if req.Method == "GET" {
							stats := map[string]uint64{
								"compareAndSwapFail":    1,
								"compareAndSwapSuccess": 2,
								"createFail":            3,
								"createSuccess":         4,
								"deleteFail":            5,
								"deleteSuccess":         6,
								"expireCount":           7,
								"getsFail":              8,
								"getsSuccess":           9,
								"setsFail":              10,
								"setsSuccess":           11,
								"updateFail":            12,
								"updateSuccess":         13,
								"watchers":              14,
							}

							statsPayload, err := json.Marshal(stats)
							Expect(err).NotTo(HaveOccurred())

							w.Write(statsPayload)
							return
						}
					case "/v2/keys/":
						if req.Method == "GET" {
							w.Header().Set("X-Etcd-Index", "10001")
							w.Header().Set("X-Raft-Index", "10204")
							w.Header().Set("X-Raft-Term", "1234")
							w.WriteHeader(http.StatusOK)
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))
				store = instruments.NewStore(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("should return them", func() {
				context := store.Emit()

				Expect(context.Name).Should(Equal("store"))
				Expect(fakeGetter.GetCall.CallCount).To(Equal(2))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "EtcdIndex",
					Value: uint64(10001),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "RaftIndex",
					Value: uint64(10204),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "RaftTerm",
					Value: uint64(1234),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "CompareAndSwapFail",
					Value: uint64(1),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "CompareAndSwapSuccess",
					Value: uint64(2),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "CreateFail",
					Value: uint64(3),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "CreateSuccess",
					Value: uint64(4),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "DeleteFail",
					Value: uint64(5),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "DeleteSuccess",
					Value: uint64(6),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "ExpireCount",
					Value: uint64(7),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "GetsFail",
					Value: uint64(8),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "GetsSuccess",
					Value: uint64(9),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "SetsFail",
					Value: uint64(10),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "SetsSuccess",
					Value: uint64(11),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "UpdateFail",
					Value: uint64(12),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "UpdateSuccess",
					Value: uint64(13),
				}))

				Expect(context.Metrics).Should(ContainElement(instrumentation.Metric{
					Name:  "Watchers",
					Value: uint64(14),
				}))
			})
		})

		Context("when the etcd server gives invalid JSON", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/stats/store":
						if req.Method == "GET" {
							w.Write([]byte("ß"))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))
				store = instruments.NewStore(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := store.Emit()
				Expect(context.Metrics).Should(BeEmpty())
			})
		})

		Context("when getting the keys fails", func() {
			BeforeEach(func() {
				etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case "/v2/keys":
						if req.Method == "GET" {
							w.WriteHeader(http.StatusNotFound)
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
				}))
				store = instruments.NewStore(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
			})

			It("does not report any metrics", func() {
				context := store.Emit()
				Expect(context.Metrics).Should(BeEmpty())
			})
		})
	})

	Context("when the metrics fail to fetch", func() {
		BeforeEach(func() {
			etcdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}))
			store = instruments.NewStore(fakeGetter, etcdServer.URL, lagertest.NewTestLogger("test"))
		})

		It("should not return them", func() {
			context := store.Emit()
			Expect(context.Metrics).Should(BeEmpty())
		})
	})
})
