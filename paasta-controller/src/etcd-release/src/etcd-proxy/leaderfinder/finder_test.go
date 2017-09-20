package leaderfinder_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcd-proxy/leaderfinder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeGetter struct {
	GetCall struct {
		CallCount int
		Returns   struct {
			Error error
		}
	}
}

func (g *fakeGetter) Get(url string) (resp *http.Response, err error) {
	g.GetCall.CallCount++

	if g.GetCall.Returns.Error != nil && strings.HasSuffix(url, "/v2/stats/self") {
		return &http.Response{}, g.GetCall.Returns.Error
	}

	return http.Get(url)
}

var _ = Describe("Finder", func() {
	var (
		getter *fakeGetter
	)

	BeforeEach(func() {
		getter = &fakeGetter{}
	})

	Describe("Find", func() {
		It("finds the the leader in an etcd cluster", func() {
			var (
				node1Server *httptest.Server
				node2URL    string
				node3URL    string
			)

			node2URL = "https:\\dummy2:4002"
			node3URL = "https:\\dummy3:4002"

			node1Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/members":
					w.Write([]byte(fmt.Sprintf(`{
					  "members": [
						{
						  "clientURLs": [
						  %q
						  ],
						  "name": "etcd-z1-0",
						  "id": "1b8722e8a026db8e"
						},
						{
						  "clientURLs": [
						  %q
						  ],
						  "name": "etcd-z1-1",
						  "id": "2ff908d1599e9e72"
						},
						{
						  "clientURLs": [
						  %q
						  ],
						  "name": "etcd-z1-2",
						  "id": "7be499c93624e6d5"
						}
					  ]
					}`, node1Server.URL, node2URL, node3URL)))
					return
				case "/v2/stats/self":
					w.Write([]byte(`{
					  "name": "etcd-z1-0",
					  "id": "1b8722e8a026db8e",
					  "state": "StateFollower",
					  "leaderInfo": {
						"leader": "2ff908d1599e9e72"
					  }
					}`))
					return
				}

				w.WriteHeader(http.StatusTeapot)
			}))

			finder := leaderfinder.NewFinder(node1Server.URL, getter)

			leader, err := finder.Find()
			Expect(err).NotTo(HaveOccurred())

			Expect(leader.String()).To(Equal(node2URL))
			Expect(getter.GetCall.CallCount).To(Equal(2))
		})

		Context("failure cases", func() {
			It("returns an error if no address has been provided", func() {
				finder := leaderfinder.NewFinder("", getter)

				_, err := finder.Find()
				Expect(err).To(MatchError("no address provided"))
			})

			It("returns an error when the call to /v2/members fails", func() {
				finder := leaderfinder.NewFinder("%%%%%%%", getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape \"%%%\"")))
			})

			It("returns an error when the call to /v2/members returns malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`%%%%%%%`))
				}))
				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
			})

			It("returns an error when no members have been found", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(`{
						  "members": []
						}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(leaderfinder.MembersNotFound))
			})

			It("returns an error when no member clientURLs have been found", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(`{
						  "members": [
							{
							  "name": "etcd-z1-0",
							  "id": "1b8722e8a026db8e"
							}
						  ]
						}`))
						return
					case "/v2/stats/self":
						w.Write([]byte(`{
					  "name": "etcd-z1-0",
					  "id": "1b8722e8a026db8e",
					  "state": "StateFollower",
					  "leaderInfo": {
						"leader": "1b8722e8a026db8e"
					  }
					}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(leaderfinder.NoClientURLsForLeader))
			})

			It("returns an error when the call to /v2/stats/self fails", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(`{
						  "members": [
							{
							  "clientURLs": [
							  	"%%%%%%%%%%"
							  ],
							  "name": "etcd-z1-0",
							  "id": "1b8722e8a026db8e"
							}
						  ]
						}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				getter.GetCall.Returns.Error = errors.New("some http error")

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError("some http error"))
			})

			It("returns an error when the call to /v2/stats/self returns malformed json", func() {
				var server *httptest.Server

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(fmt.Sprintf(`{
						  "members": [
							{
							  "clientURLs": [
							  	%q
							  ],
							  "name": "etcd-z1-0",
							  "id": "1b8722e8a026db8e"
							}
						  ]
						}`, server.URL)))
						return
					case "/v2/stats/self":
						w.Write([]byte(`%%%%%`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
			})

			It("returns an error if the leader does not have a client url", func() {
				var server *httptest.Server
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(fmt.Sprintf(`{
						  "members": [
							{
							  "clientURLs": [
							  	%q
							  ],
							  "name": "etcd-z1-1",
							  "id": "1b8722e8a026db8e"
							},
							{
							  "clientURLs": [],
							  "name": "etcd-z1-0",
							  "id": "2ff908d1599e9e72"
							}
						  ]
						}`, server.URL)))
						return
					case "/v2/stats/self":
						w.Write([]byte(`{
						  "name": "etcd-z1-0",
						  "id": "1b8722e8a026db8e",
						  "state": "StateFollower",
						  "leaderInfo": {
							"leader": "2ff908d1599e9e72"
						  }
						}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(leaderfinder.NoClientURLsForLeader))
			})

			It("returns a LeaderNotFound error when a leader cannot be found", func() {
				var server *httptest.Server

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(fmt.Sprintf(`{
						  "members": [
							{
							  "clientURLs": [
							  	%q
							  ],
							  "name": "etcd-z1-0",
							  "id": "1b8722e8a026db8e"
							}
						  ]
						}`, server.URL)))
						return
					case "/v2/stats/self":
						w.Write([]byte(`{
						  "name": "etcd-z1-0",
						  "id": "1b8722e8a026db8e",
						  "state": "StateFollower",
						  "leaderInfo": {
							"leader": "2ff908d1599e9e72"
						  }
						}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(leaderfinder.LeaderNotFound))
			})

			It("returns an error when the leader client url is malformed", func() {
				var server *httptest.Server

				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/v2/members":
						w.Write([]byte(fmt.Sprintf(`{
						  "members": [
							{
							  "clientURLs": [
							  	%q
							  ],
							  "name": "etcd-z1-0",
							  "id": "1b8722e8a026db8e"
							},
							{
							  "clientURLs": [
							  	"%%somebadurl%%"
							  ],
							  "name": "etcd-z1-1",
							  "id": "2ff908d1599e9e72"
							}
						  ]
						}`, server.URL)))
						return
					case "/v2/stats/self":
						w.Write([]byte(`{
						  "name": "etcd-z1-0",
						  "id": "1b8722e8a026db8e",
						  "state": "StateFollower",
						  "leaderInfo": {
							"leader": "2ff908d1599e9e72"
						  }
						}`))
						return
					}

					w.WriteHeader(http.StatusTeapot)
				}))

				finder := leaderfinder.NewFinder(server.URL, getter)

				_, err := finder.Find()
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})
		})
	})
})
