package etcd_test

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("etcd", func() {
	AfterEach(func() {
		etcd.ResetBodyReader()
	})

	Describe("Address", func() {
		It("returns the client address", func() {
			client := etcd.NewClient("http://some-address")
			Expect(client.Address()).To(Equal("http://some-address"))
		})
	})

	Describe("Get", func() {
		It("returns the value with the given key", func() {
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/kv/some-key" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("some-value"))
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			}))

			client := etcd.NewClient(testServer.URL)

			value, err := client.Get("some-key")

			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal("some-value"))
		})

		Context("failure cases", func() {
			It("returns an error when the request fails", func() {
				client := etcd.NewClient("%%%%%%")

				_, err := client.Get("some-key")
				Expect(err.(*url.Error).Op).To(Equal("parse"))
			})

			It("returns an error when a bad read occurs", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("some-value"))
				}))

				etcd.SetBodyReader(func(io.Reader) ([]byte, error) {
					return []byte{}, errors.New("bad things happened")
				})

				client := etcd.NewClient(testServer.URL)

				_, err := client.Get("some-key")
				Expect(err).To(MatchError("bad things happened"))
			})

			It("returns an error when the response is not 200", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("something bad happened"))
				}))

				client := etcd.NewClient(testServer.URL)

				_, err := client.Get("some-key")
				Expect(err).To(MatchError("unexpected status: 500 Internal Server Error something bad happened"))
			})
		})
	})

	Describe("Set", func() {
		It("sets the value with the given key", func() {
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := ioutil.ReadAll(r.Body)
				Expect(err).NotTo(HaveOccurred())

				if r.Method == "PUT" && r.URL.Path == "/kv/some-key" {
					if string(body) == "some-value" {
						w.WriteHeader(http.StatusCreated)
						return
					}
				}
				w.WriteHeader(http.StatusInternalServerError)
			}))

			client := etcd.NewClient(testServer.URL)

			err := client.Set("some-key", "some-value")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("failure cases", func() {
			It("returns an error when the status is not StatusCreated", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("something bad happened"))
				}))

				client := etcd.NewClient(testServer.URL)

				err := client.Set("some-key", "some-value")
				Expect(err).To(MatchError("unexpected status: 500 Internal Server Error something bad happened"))
			})

			It("returns an error when the request is malformed", func() {
				client := etcd.NewClient("%%%%%")

				err := client.Set("some-key", "some-value")
				Expect(err.(*url.Error).Op).To(Equal("parse"))
			})

			It("returns an error when the request is malformed", func() {
				client := etcd.NewClient("banana://something")

				err := client.Set("some-key", "some-value")
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("returns an error when a bad read occurs", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
				}))

				etcd.SetBodyReader(func(io.Reader) ([]byte, error) {
					return []byte{}, errors.New("bad things happened")
				})

				client := etcd.NewClient(testServer.URL)

				err := client.Set("some-key", "some-value")
				Expect(err).To(MatchError("bad things happened"))
			})
		})
	})

	Describe("Leader", func() {
		It("returns the name of the cluster leader", func() {
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/leader" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("etcd-z1-2"))
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			}))

			client := etcd.NewClient(testServer.URL)

			leaderName, err := client.Leader()
			Expect(err).NotTo(HaveOccurred())

			Expect(leaderName).To(Equal("etcd-z1-2"))
		})

		Context("failure cases", func() {
			It("returns an error when the request fails", func() {
				client := etcd.NewClient("%%%%%%")

				_, err := client.Leader()
				Expect(err.(*url.Error).Op).To(Equal("parse"))
			})

			It("returns an error when a bad read occurs", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("some-value"))
				}))

				etcd.SetBodyReader(func(io.Reader) ([]byte, error) {
					return []byte{}, errors.New("bad things happened")
				})

				client := etcd.NewClient(testServer.URL)

				_, err := client.Leader()
				Expect(err).To(MatchError("bad things happened"))
			})

			It("returns an error when the response is not 200", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("something bad happened"))
				}))

				client := etcd.NewClient(testServer.URL)

				_, err := client.Leader()
				Expect(err).To(MatchError("unexpected status: 500 Internal Server Error something bad happened"))
			})
		})
	})
})
