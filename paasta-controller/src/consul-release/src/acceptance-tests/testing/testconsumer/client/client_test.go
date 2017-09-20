package client_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/testconsumer/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("client", func() {
	Describe("SetHealthCheck", func() {
		It("sets the health of the check", func() {
			wasCalled := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/health_check" && req.Method == "POST" {
					buf, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())
					if string(buf) == "false" {
						wasCalled = true
						w.WriteHeader(http.StatusOK)
						return
					}

					w.WriteHeader(http.StatusInternalServerError)
				}

				w.WriteHeader(http.StatusTeapot)
			}))

			tcClient := client.New(server.URL)

			err := tcClient.SetHealthCheck(false)
			Expect(err).NotTo(HaveOccurred())

			Expect(wasCalled).To(BeTrue())
		})

		Context("failure cases", func() {
			It("returns an error when the url is malformed", func() {
				tcClient := client.New("%%%%%%")

				err := tcClient.SetHealthCheck(true)
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("returns an error when the response is not 200", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(http.StatusTeapot)
					fmt.Fprint(w, "something bad happened")
				}))

				tcClient := client.New(server.URL)

				err := tcClient.SetHealthCheck(true)
				Expect(err).To(MatchError("unexpected status: 418 I'm a teapot something bad happened"))
			})
		})
	})

	Describe("DNS", func() {
		AfterEach(func() {
			client.ResetBodyReader()
		})

		It("returns a slice of ip addresses given a service", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/dns" && req.Method == "GET" {
					if req.URL.Query().Get("service") == "some-service-name" {
						fmt.Fprint(w, `["127.0.0.2","127.0.0.3","127.0.0.4"]`)
						return
					}
				}

				w.WriteHeader(http.StatusTeapot)
			}))

			tcClient := client.New(server.URL)

			addresses, err := tcClient.DNS("some-service-name")
			Expect(err).NotTo(HaveOccurred())

			Expect(addresses).To(Equal([]string{"127.0.0.2", "127.0.0.3", "127.0.0.4"}))
		})

		Context("failure cases", func() {
			It("returns an error when the url is malformed", func() {
				tcClient := client.New("%%%%%%")

				_, err := tcClient.DNS("some-service-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("returns an error when the response body cannot be read", func() {
				client.SetBodyReader(func(io.Reader) ([]byte, error) {
					return []byte{}, errors.New("failed to read body")
				})

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {}))

				tcClient := client.New(server.URL)

				_, err := tcClient.DNS("some-service-name")
				Expect(err).To(MatchError("failed to read body"))
			})

			It("returns an error when the response is not 200", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(http.StatusTeapot)
					fmt.Fprint(w, "something bad happened")
				}))

				tcClient := client.New(server.URL)

				_, err := tcClient.DNS("some-service-name")
				Expect(err).To(MatchError("unexpected status: 418 I'm a teapot something bad happened"))
			})

			It("returns an error when the json response is malformed", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					fmt.Fprint(w, "%%%%%%%%%%%")
				}))

				tcClient := client.New(server.URL)

				_, err := tcClient.DNS("some-service-name")
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})
})
