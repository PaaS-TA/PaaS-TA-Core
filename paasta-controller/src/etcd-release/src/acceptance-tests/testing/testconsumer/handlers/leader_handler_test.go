package handlers_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/testconsumer/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LeaderHandler", func() {
	var (
		handler      handlers.LeaderHandler
		happyHandler http.HandlerFunc
	)

	Describe("ServeHTTP", func() {
		BeforeEach(func() {
			happyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/stats/self":
					if r.Method == "GET" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
								"leaderInfo": {
								  "startTime": "2016-09-20T20:41:29.990832596Z",
								  "uptime": "20m3.379868254s",
								  "leader": "a63914b93e51e236"
								}
							}`))
						return
					}
				case "/v2/members":
					if r.Method == "GET" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
								"members": [
								  {
									"clientURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-0",
									"id": "1b8722e8a026db8e"
								  },
								  {
									"clientURLs": [
									  "https://etcd-z1-1.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-1.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-1",
									"id": "9aac0801933fa6e0"
								  },
								  {
									"clientURLs": [
									  "https://etcd-z1-2.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-2.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-2",
									"id": "a63914b93e51e236"
								  }
								]
							}`))
						return
					}
				default:
					w.WriteHeader(http.StatusTeapot)
					return
				}
			})
		})
		Context("ssl etcd", func() {
			var (
				etcdServer *httptest.Server
			)

			BeforeEach(func() {
				var err error

				etcdServer = httptest.NewUnstartedServer(happyHandler)
				etcdServer.TLS = &tls.Config{}

				etcdServer.TLS.Certificates = make([]tls.Certificate, 1)
				etcdServer.TLS.Certificates[0], err = tls.LoadX509KeyPair("../fixtures/server.crt", "../fixtures/server.key")
				Expect(err).NotTo(HaveOccurred())

				etcdServer.StartTLS()
			})

			It("returns the node name of the current leader", func() {
				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "../fixtures/ca.crt", "../fixtures/client.crt", "../fixtures/client.key")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal("etcd-z1-2"))
			})

			Context("when a node url is specified", func() {
				It("returns the node name of the leader a given node knows about", func() {
					otherEtcdServer := httptest.NewUnstartedServer(
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							switch r.URL.Path {
							case "/v2/stats/self":
								if r.Method == "GET" {
									w.WriteHeader(http.StatusOK)
									w.Write([]byte(`{
										"leaderInfo": {
										  "startTime": "2016-09-20T20:41:29.990832596Z",
										  "uptime": "20m3.379868254s",
										  "leader": "1b8722e8a026db8e"
										}
									}`))
									return
								}
							case "/v2/members":
								if r.Method == "GET" {
									w.WriteHeader(http.StatusOK)
									w.Write([]byte(`{
										"members": [
										  {
											"clientURLs": [
											  "https://etcd-z1-0.etcd.service.cf.internal:4001"
											],
											"peerURLs": [
											  "https://etcd-z1-0.etcd.service.cf.internal:7001"
											],
											"name": "etcd-z1-0",
											"id": "1b8722e8a026db8e"
										  },
										  {
											"clientURLs": [
											  "https://etcd-z1-1.etcd.service.cf.internal:4001"
											],
											"peerURLs": [
											  "https://etcd-z1-1.etcd.service.cf.internal:7001"
											],
											"name": "etcd-z1-1",
											"id": "9aac0801933fa6e0"
										  }
										]
									}`))
									return
								}
							default:
								w.WriteHeader(http.StatusTeapot)
								return
							}
						}),
					)
					otherEtcdServer.TLS = &tls.Config{}

					otherEtcdServer.TLS.Certificates = make([]tls.Certificate, 1)
					var err error
					otherEtcdServer.TLS.Certificates[0], err = tls.LoadX509KeyPair("../fixtures/server.crt", "../fixtures/server.key")
					Expect(err).NotTo(HaveOccurred())

					otherEtcdServer.StartTLS()

					parts := strings.Split(otherEtcdServer.URL, ":")

					request, err := http.NewRequest("GET", "/", nil)
					Expect(err).NotTo(HaveOccurred())

					request.URL.RawQuery = "node=https%3A%2F%2F127.0.0.1%3A" + parts[2]

					handler = handlers.NewLeaderHandler(etcdServer.URL, "../fixtures/ca.crt", "../fixtures/client.crt", "../fixtures/client.key")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusOK))
					Expect(recorder.Body.String()).To(Equal("etcd-z1-0"))
				})
			})

			Context("failure cases", func() {
				It("returns a 500 when the provided cert is not a valid path", func() {
					request, err := http.NewRequest("GET", "/", nil)
					Expect(err).NotTo(HaveOccurred())

					fakePath := "/some/fake/path"
					handler = handlers.NewLeaderHandler(etcdServer.URL, fakePath, fakePath, fakePath)

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("no such file or directory"))
				})

				It("returns a 500 when the provided ca is not a valid path", func() {
					request, err := http.NewRequest("GET", "/", nil)
					Expect(err).NotTo(HaveOccurred())

					fakePath := "/some/fake/path"
					handler = handlers.NewLeaderHandler(etcdServer.URL, fakePath, "../fixtures/client.crt", "../fixtures/client.key")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("no such file or directory"))
				})
			})
		})

		Context("non-ssl etcd", func() {
			var (
				etcdServer *httptest.Server
			)

			BeforeEach(func() {
				etcdServer = httptest.NewServer(happyHandler)
			})

			It("returns the node name of the current leader", func() {
				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal("etcd-z1-2"))
			})
		})

		Context("failure cases", func() {
			It("returns a 500 when the etcd url is malformed", func() {
				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler("%%%%%%%", "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("invalid URL escape"))
			})

			It("returns a 500 when the members could not be retrieved", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/self":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte(`{
								"leaderInfo": {
								  "startTime": "2016-09-20T20:41:29.990832596Z",
								  "uptime": "20m3.379868254s",
								  "leader": "a63914b93e51e236"
								}
							}`))
							return
						}
					case "/v2/members":
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte("error getting members"))
						return
					default:
						w.WriteHeader(http.StatusTeapot)
						return
					}
				}))

				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("unexpected status code 500 - error getting members"))
			})

			It("returns a 500 when members returns malformed json", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/self":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte(`{
								"leaderInfo": {
								  "startTime": "2016-09-20T20:41:29.990832596Z",
								  "uptime": "20m3.379868254s",
								  "leader": "a63914b93e51e236"
								}
							}`))
							return
						}
					case "/v2/members":
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("%%%%%%"))
						return
					default:
						w.WriteHeader(http.StatusTeapot)
						return
					}
				}))

				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("invalid character"))
			})

			It("returns a 500 when the stats could not be retrieved", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/self":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusInternalServerError)
							w.Write([]byte("error getting stats"))
							return
						}
					case "/v2/members":
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
								"members": [
								  {
									"clientURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-0",
									"id": "1b8722e8a026db8e"
								  }
								]
							}`))
						return
					default:
						w.WriteHeader(http.StatusTeapot)
						return
					}
				}))

				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("unexpected status code 500 - error getting stats"))
			})

			It("returns a 500 when the stats contains malformed json", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/self":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte("%%%%%%%%%%"))
							return
						}
					case "/v2/members":
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
								"members": [
								  {
									"clientURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-0",
									"id": "1b8722e8a026db8e"
								  }
								]
							}`))
						return
					default:
						w.WriteHeader(http.StatusTeapot)
						return
					}
				}))

				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("invalid character"))
			})

			It("returns a 500 when the leader could not be determined", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/self":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte(`{
								"leaderInfo": {
								  "startTime": "2016-09-20T20:41:29.990832596Z",
								  "uptime": "20m3.379868254s",
								  "leader": "YYYYYYYYYYYYYYYYYYYYYY"
								}
							}`))
							return
						}
					case "/v2/members":
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
								"members": [
								  {
									"clientURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:4001"
									],
									"peerURLs": [
									  "https://etcd-z1-0.etcd.service.cf.internal:7001"
									],
									"name": "etcd-z1-0",
									"id": "XXXXXXXXXXXXXXXXXXXXX"
								  }
								]
							}`))
						return
					default:
						w.WriteHeader(http.StatusTeapot)
						return
					}
				}))

				request, err := http.NewRequest("GET", "/", nil)
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewLeaderHandler(etcdServer.URL, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(ContainSubstring("could not determine leader"))
			})
		})
	})
})
