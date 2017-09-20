package handlers_test

import (
	"acceptance-tests/testing/testconsumer/handlers"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KVHandler", func() {
	var (
		handler handlers.KVHandler
	)

	Describe("ServeHTTP", func() {
		Context("ssl etcd client", func() {
			var (
				etcdServer *httptest.Server
			)
			BeforeEach(func() {
				var err error

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.Method {
					case "PUT":
						body, err := ioutil.ReadAll(r.Body)
						Expect(err).NotTo(HaveOccurred())

						if r.URL.Path == "/v2/keys/some-key" && strings.Contains(string(body), "some-value") {
							w.WriteHeader(http.StatusCreated)
							w.Write([]byte(`{
								"action": "set",
								"node": {
									"createdIndex": 2,
									"key": "/some-key",
									"modifiedIndex": 2,
									"value": "some-value"
								}
							}`))
							return
						}
					case "GET":
						if r.URL.Path == "/v2/keys/some-key" {
							w.WriteHeader(http.StatusOK)

							w.Write([]byte(`{
								"action": "get",
								"node": {
									"createdIndex": 2,
									"key": "/some-key",
									"modifiedIndex": 2,
									"value": "some-value"
								}
							}`))
							return
						}
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				})

				etcdServer = httptest.NewUnstartedServer(handler)
				etcdServer.TLS = &tls.Config{}

				etcdServer.TLS.Certificates = make([]tls.Certificate, 1)
				etcdServer.TLS.Certificates[0], err = tls.LoadX509KeyPair("../fixtures/server.crt", "../fixtures/server.key")
				Expect(err).NotTo(HaveOccurred())

				etcdServer.StartTLS()
			})

			It("sets a value given a key", func() {
				request, err := http.NewRequest("PUT", "/kv/some-key", strings.NewReader("some-value"))
				Expect(err).NotTo(HaveOccurred())

				handler = handlers.NewKVHandler([]string{etcdServer.URL}, "../fixtures/ca.crt", "../fixtures/client.crt", "../fixtures/client.key")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusCreated))
			})

			It("gets a value given a key", func() {
				request, err := http.NewRequest("GET", "/kv/some-key", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())
				handler = handlers.NewKVHandler([]string{etcdServer.URL}, "../fixtures/ca.crt", "../fixtures/client.crt", "../fixtures/client.key")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusOK))
				Expect(recorder.Body.String()).To(Equal("some-value"))
			})

			Context("failure cases", func() {
				It("returns a 500 when the provided cert is not a valid path", func() {
					request, err := http.NewRequest("GET", "/kv/some-key", strings.NewReader(""))
					Expect(err).NotTo(HaveOccurred())
					fakePath := "/some/fake/path"
					handler = handlers.NewKVHandler([]string{etcdServer.URL}, fakePath, fakePath, fakePath)

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("no such file or directory"))
				})
			})
		})

		Context("non-ssl etcd client", func() {
			It("returns a StatusMethodNotAllowed when an unknown method has been used", func() {
				etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.Method {
					case "GET":
					case "PUT":
					default:
						w.WriteHeader(http.StatusMethodNotAllowed)
					}
				}))

				request, err := http.NewRequest("DELETE", "/kv/some-key", strings.NewReader(""))
				Expect(err).NotTo(HaveOccurred())
				handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(http.StatusMethodNotAllowed))
			})

			Context("GET", func() {
				It("gets a value given a key", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							if r.URL.Path == "/v2/keys/some-key" {
								w.WriteHeader(http.StatusOK)

								w.Write([]byte(`{
								"action": "get",
								"node": {
									"createdIndex": 2,
									"key": "/some-key",
									"modifiedIndex": 2,
									"value": "some-value"
								}
							}`))
							}
						default:
							w.WriteHeader(http.StatusMethodNotAllowed)
						}
					}))

					request, err := http.NewRequest("GET", "/kv/some-key", strings.NewReader(""))
					Expect(err).NotTo(HaveOccurred())
					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusOK))
					Expect(recorder.Body.String()).To(Equal("some-value"))
				})

				It("returns a 404 when the key cannot be found", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							w.WriteHeader(http.StatusNotFound)
							w.Write([]byte(`{
							"errorCode": 100,
							"message": "Key not found",
							"index": 1
						}`))
						default:
							w.WriteHeader(http.StatusMethodNotAllowed)
						}
					}))

					request, err := http.NewRequest("GET", "/", strings.NewReader(""))
					Expect(err).NotTo(HaveOccurred())
					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusNotFound))
					Expect(recorder.Body.String()).To(ContainSubstring("Key not found"))
				})

				It("returns a 500 when an unknown error occurs", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch r.Method {
						case "GET":
							w.WriteHeader(http.StatusTeapot)
							w.Write([]byte(`{
							"errorCode": 500,
							"message": "something really bad happened",
							"index": 100
						}`))
						default:
							w.WriteHeader(http.StatusMethodNotAllowed)
						}
					}))

					request, err := http.NewRequest("GET", "/kv/some-key", strings.NewReader(""))
					Expect(err).NotTo(HaveOccurred())
					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("something really bad happened"))
				})
			})

			Context("PUT", func() {
				It("sets a value given a key", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := ioutil.ReadAll(r.Body)
						if err != nil {
							panic(err)
						}

						if r.Method == "PUT" {
							if r.URL.Path == "/v2/keys/some-key" && strings.Contains(string(body), "some-value") {
								w.WriteHeader(http.StatusCreated)
								w.Write([]byte(`{
								"action": "set",
								"node": {
									"createdIndex": 2,
									"key": "/some-key",
									"modifiedIndex": 2,
									"value": "some-value"
								}
							}`))
								return
							}
						}

						w.WriteHeader(http.StatusTeapot)
					}))

					request, err := http.NewRequest("PUT", "/kv/some-key", strings.NewReader("some-value"))
					Expect(err).NotTo(HaveOccurred())

					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusCreated))
				})

				It("returns a 500 when the key cannot be written", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method == "PUT" {
							if r.URL.Path == "/v2/keys/some-key" {
								w.WriteHeader(http.StatusTeapot)
								w.Write([]byte(`{
								"errorCode": 500,
								"message": "something really bad happened",
								"index": 100
							}`))
								return
							}
						}

						w.WriteHeader(http.StatusTeapot)
					}))

					request, err := http.NewRequest("PUT", "/kv/some-key", strings.NewReader(""))
					Expect(err).NotTo(HaveOccurred())

					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("something really bad happened"))
				})

				It("returns a 500 when the body cannot be read", func() {
					etcdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					}))

					request, err := http.NewRequest("PUT", "/kv/some-key", badReader{})
					Expect(err).NotTo(HaveOccurred())

					handler = handlers.NewKVHandler([]string{etcdServer.URL}, "", "", "")

					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)

					Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
					Expect(recorder.Body.String()).To(ContainSubstring("bad read"))
				})
			})
		})
	})
})

type badReader struct{}

func (badReader) Read([]byte) (int, error) {
	return 0, errors.New("bad read")
}
