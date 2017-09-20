package app_test

import (
	"crypto/tls"
	"etcd-consistency-checker/app"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func newTLSServer(handler http.Handler) *httptest.Server {
	var err error
	etcdServer := httptest.NewUnstartedServer(handler)
	etcdServer.TLS = &tls.Config{}

	etcdServer.TLS.Certificates = make([]tls.Certificate, 1)
	etcdServer.TLS.Certificates[0], err = tls.LoadX509KeyPair("../fixtures/server.crt", "../fixtures/server.key")
	Expect(err).NotTo(HaveOccurred())

	etcdServer.StartTLS()
	return etcdServer
}

var _ = Describe("App", func() {
	Context("Run", func() {
		DescribeTable("returns an error when more than one leader exists",
			func(newServer func(http.Handler) *httptest.Server, ca, cert, key string) {
				etcdServer1Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte(`{
							"leader": "XXXXXXXXXXXXXXXXXXXXXX"
						}`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				})

				etcdServer2CallCount := 0
				etcdServer2Handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							if etcdServer2CallCount >= 3 {
								w.WriteHeader(http.StatusOK)
								w.Write([]byte(`{
								"leader": "YYYYYYYYYYYYYYYYYYYYY"
							}`))
							} else {
								w.WriteHeader(http.StatusForbidden)
								w.Write([]byte(`{
								 "message": "not current leader"
							}`))
							}
							etcdServer2CallCount++
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				})

				var sleeperCallDuration time.Duration
				sleeperCallCount := 0

				etcdServer1 := newServer(etcdServer1Handler)
				etcdServer2 := newServer(etcdServer2Handler)
				a := app.New(app.Config{
					ClusterMembers: []string{
						etcdServer1.URL,
						etcdServer2.URL,
					},
					CA:   ca,
					Cert: cert,
					Key:  key,
				},
					func(d time.Duration) {
						sleeperCallCount++
						sleeperCallDuration = d
					},
				)

				err := a.Run()
				Expect(err).To(MatchError(fmt.Sprintf("more than one leader exists: [%s %s]", etcdServer1.URL, etcdServer2.URL)))
				Expect(etcdServer2CallCount).To(Equal(4))
				Expect(sleeperCallCount).To(Equal(3))
				Expect(sleeperCallDuration).To(Equal(1 * time.Second))
			},
			Entry("when tls is enabled", newTLSServer, "../fixtures/ca.crt", "../fixtures/client.crt", "../fixtures/client.key"),
			Entry("when tls is not enabled", httptest.NewServer, "", "", ""),
		)

		It("filters connection errors to the cluster members", func() {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/stats/leader":
					if r.Method == "GET" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{
							"leader": "XXXXXXXXXXXXXXXXXXXXXX"
						}`))
						return
					}
				}
				w.WriteHeader(http.StatusTeapot)
				return
			})

			etcdServer1 := httptest.NewServer(handler)
			etcdServer2 := httptest.NewServer(handler)
			a := app.New(app.Config{
				ClusterMembers: []string{
					etcdServer1.URL,
					etcdServer2.URL,
					"http://some.fake.domain",
					"http://127.0.0.1:12345",
				},
			},
				func(d time.Duration) {},
			)

			err := a.Run()
			Expect(err).To(MatchError(fmt.Sprintf("more than one leader exists: [%s %s]", etcdServer1.URL, etcdServer2.URL)))
		})

		Context("failure cases", func() {
			var sleeper func(time.Duration)

			BeforeEach(func() {
				sleeper = func(time.Duration) {}
			})

			It("returns an error when there are no cluster members", func() {
				a := app.New(app.Config{
					ClusterMembers: []string{},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError("at least one cluster member is required"))
			})

			It("returns an error when the cert and key don't exist", func() {
				a := app.New(app.Config{
					ClusterMembers: []string{
						"http://something.com",
					},
					CA:   "../fixtures/ca.crt",
					Cert: "/some/fake/path",
					Key:  "/some/fake/path",
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})

			It("returns an error when the ca doesn't exist", func() {
				a := app.New(app.Config{
					ClusterMembers: []string{
						"http://something.com",
					},
					CA:   "/some/fake/path",
					Cert: "../fixtures/client.crt",
					Key:  "../fixtures/client.key",
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})

			It("returns an error when the cluster member url is malformed", func() {
				a := app.New(app.Config{
					ClusterMembers: []string{
						"%%%%%%%%",
					},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("returns an error when the stats/leader returns malformed json for a leader node", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusOK)
							w.Write([]byte(`%%%%%%%%%%%%%%%%%%%%%%%`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				}))

				a := app.New(app.Config{
					ClusterMembers: []string{server.URL},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("returns an error when the stats/leader returns malformed json for a non leader node", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusForbidden)
							w.Write([]byte(`%%%%%%%%%%%%%%%%%%%%%%%`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				}))

				a := app.New(app.Config{
					ClusterMembers: []string{server.URL},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("returns an error when the stats/leader returns an unknown message for a non leader node", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusForbidden)
							w.Write([]byte(`{"message": "unknown"}`))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				}))

				a := app.New(app.Config{
					ClusterMembers: []string{server.URL},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring(`unexpected status code 403 - {"message": "unknown"}`)))
			})

			It("returns an error when the stats/leader returns an unknown status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/stats/leader":
						if r.Method == "GET" {
							w.WriteHeader(http.StatusInternalServerError)
							w.Write([]byte("something bad happened"))
							return
						}
					}
					w.WriteHeader(http.StatusTeapot)
					return
				}))

				a := app.New(app.Config{
					ClusterMembers: []string{server.URL},
				}, sleeper)

				err := a.Run()
				Expect(err).To(MatchError(ContainSubstring("unexpected status code 500 - something bad happened")))
			})
		})
	})
})
