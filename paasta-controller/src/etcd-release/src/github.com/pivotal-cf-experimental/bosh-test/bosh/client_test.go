package bosh_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/gomegamatchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("client", func() {
	Context("GetTaskOutput", func() {
		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/tasks/1/output"))
				Expect(r.URL.RawQuery).To(Equal("type=event"))
				Expect(r.Method).To(Equal("GET"))

				w.Write([]byte(`
				{"time": 0, "error": {"code": 100, "message": "some-error" }, "stage": "some-stage", "tags": [ "some-tag" ], "total": 1, "task": "some-task-guid", "index": 1, "state": "some-state", "progress": 0}
{"time": 1, "error": {"code": 100, "message": "some-error" }, "stage": "some-stage", "tags": [ "some-tag" ], "total": 1, "task": "some-task-guid", "index": 1, "state": "some-new-state", "progress": 0}
				`))
			}))
		})

		It("returns task event output for a given task", func() {
			client := bosh.NewClient(bosh.Config{
				URL:      server.URL,
				Username: "some-username",
				Password: "some-password",
			})

			taskOutputs, err := client.GetTaskOutput(1)
			Expect(err).NotTo(HaveOccurred())
			Expect(taskOutputs).To(ConsistOf(
				bosh.TaskOutput{
					Time: 0,
					Error: bosh.TaskError{
						Code:    100,
						Message: "some-error",
					},
					Stage:    "some-stage",
					Tags:     []string{"some-tag"},
					Total:    1,
					Task:     "some-task-guid",
					Index:    1,
					State:    "some-state",
					Progress: 0,
				},
				bosh.TaskOutput{
					Time: 1,
					Error: bosh.TaskError{
						Code:    100,
						Message: "some-error",
					},
					Stage:    "some-stage",
					Tags:     []string{"some-tag"},
					Total:    1,
					Task:     "some-task-guid",
					Index:    1,
					State:    "some-new-state",
					Progress: 0,
				},
			))
		})

		Context("failure cases", func() {
			It("error on a malformed URL", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "%%%%%%%%",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.GetTaskOutput(1)
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("error on an empty URL", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.GetTaskOutput(1)
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("errors on an unexpected status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.GetTaskOutput(1)
				Expect(err).To(MatchError("unexpected response 502 Bad Gateway"))
			})

			It("should error on a bogus response body", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				bosh.SetBodyReader(func(io.Reader) ([]byte, error) {
					return nil, errors.New("a bad read happened")
				})

				_, err := client.GetTaskOutput(1)
				Expect(err).To(MatchError("a bad read happened"))
			})

			It("error on malformed JSON", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`%%%%%%%%`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.GetTaskOutput(1)
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})

	Context("Deployments", func() {
		It("retrieves all deployments from the director", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/deployments"))
				Expect(r.Method).To(Equal("GET"))

				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				w.Write([]byte(`[
					{"name": "deployment1"},
					{"name": "deployment2"},
					{"name": "deployment3"}
				]`))
			}))

			client := bosh.NewClient(bosh.Config{
				URL:      server.URL,
				Username: "some-username",
				Password: "some-password",
			})

			deployments, err := client.Deployments()
			Expect(err).NotTo(HaveOccurred())
			Expect(deployments).To(Equal(
				[]bosh.Deployment{
					{
						Name: "deployment1",
					},
					{
						Name: "deployment2",
					},
					{
						Name: "deployment3",
					},
				},
			))
		})

		Context("failure cases", func() {
			It("error on a malformed URL", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "%%%%%%%%",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Deployments()
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("error on an empty URL", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Deployments()
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("errors on an unexpected status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Deployments()
				Expect(err).To(MatchError("unexpected response 502 Bad Gateway"))
			})

			It("error on malformed JSON", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`%%%%%%%%`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Deployments()
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})

	Context("ScanAndFix", func() {
		It("scans and fixes all instances in a deployment", func() {
			var callCount int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/deployments/some-deployment-name/scan_and_fix":
					Expect(r.Method).To(Equal("PUT"))
					Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					body, err := ioutil.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					defer r.Body.Close()

					Expect(string(body)).To(MatchJSON(`{
						"jobs":{
							"consul_z1": [0,1],
							"consul_z3": [0]
						}
					}`))
					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
				case "/tasks/1":
					Expect(r.Method).To(Equal("GET"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					if callCount == 3 {
						w.Write([]byte(`{"state": "done"}`))
					} else {
						w.Write([]byte(`{"state": "processing"}`))
					}
					callCount++
				default:
					Fail("unexpected route")
				}
			}))

			client := bosh.NewClient(bosh.Config{
				URL:                 server.URL,
				Username:            "some-username",
				Password:            "some-password",
				TaskPollingInterval: time.Nanosecond,
			})

			yaml := `---
name: some-deployment-name
jobs:
  - name: consul_z1
    instances: 2
  - name: consul_z2
    instances: 0
  - name: consul_z3
    instances: 1
`

			err := client.ScanAndFix([]byte(yaml))
			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(4))
		})

		Context("failure cases", func() {
			It("errors on malformed yaml", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "http://example.com",
					Username: "some-username",
					Password: "some-password",
				})

				err := client.ScanAndFix([]byte("%%%%%%%%%%%%%%%"))
				Expect(err).To(MatchError(ContainSubstring("yaml: ")))
			})

			It("errors when the bosh URL is malformed", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "banana://example.com",
					Username: "some-username",
					Password: "some-password",
				})

				err := client.ScanAndFix([]byte("---\njobs: []"))
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("errors when the deployment name contains invalid URL characters", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "http://example.com%%%%%%%%%",
					Username: "some-username",
					Password: "some-password",
				})

				err := client.ScanAndFix([]byte("---\njobs: []"))
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("errors when the redirect location is bad", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", "%%%%%%%%%%%")
					w.WriteHeader(http.StatusFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				err := client.ScanAndFix([]byte("---\njobs: []"))
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("errors when the response is not a redirect", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				err := client.ScanAndFix([]byte("---\njobs: []"))
				Expect(err).To(MatchError("unexpected response 400 Bad Request"))
			})
		})
	})

	Context("Stemcell", func() {
		It("fetches the stemcell from the director", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/stemcells"))
				Expect(r.Method).To(Equal("GET"))

				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				w.Write([]byte(`[
					{"name": "some-stemcell-name","version": "1"},
					{"name": "some-stemcell-name","version": "2"},
					{"name": "some-other-stemcell-name","version": "100"}
				]`))

			}))

			client := bosh.NewClient(bosh.Config{
				URL:      server.URL,
				Username: "some-username",
				Password: "some-password",
			})

			stemcell, err := client.Stemcell("some-stemcell-name")

			Expect(err).NotTo(HaveOccurred())
			Expect(stemcell.Name).To(Equal("some-stemcell-name"))
			Expect(stemcell.Versions).To(Equal([]string{"1", "2"}))
		})

		Context("failure cases", func() {
			It("should error on a non 200 status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/stemcells"))
					w.WriteHeader(http.StatusBadRequest)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Stemcell("some-stemcell-name")

				Expect(err).To(MatchError("unexpected response 400 Bad Request"))
			})

			It("should error with a helpful message on 404 status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/stemcells"))
					w.WriteHeader(http.StatusNotFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Stemcell("some-stemcell-name")

				Expect(err).To(MatchError("stemcell some-stemcell-name could not be found"))
			})

			It("should error on an unsupported protocol", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "banana://example.com",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Stemcell("some-stemcell-name")
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("should error on a malformed url", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "&&&&&%%%&%&%&%&%&",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Stemcell("some-stemcell-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error on malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`&&%%%%%&%&%&%&%&%&%&%&`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Stemcell("some-stemcell-name")
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})

	Context("Release", func() {
		It("fetches the release from the director", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/releases/some-release-name"))
				Expect(r.Method).To(Equal("GET"))

				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				w.Write([]byte(`{"versions":["some-version","some-version.1","some-version.2"]}`))
			}))

			client := bosh.NewClient(bosh.Config{
				URL:      server.URL,
				Username: "some-username",
				Password: "some-password",
			})

			release, err := client.Release("some-release-name")

			Expect(err).NotTo(HaveOccurred())
			Expect(release.Name).To(Equal("some-release-name"))
			Expect(release.Versions).To(Equal([]string{"some-version", "some-version.1", "some-version.2"}))
		})

		Context("failure cases", func() {
			It("should error on a non 200 status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/releases/some-release-name"))
					w.WriteHeader(http.StatusBadRequest)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Release("some-release-name")

				Expect(err).To(MatchError("unexpected response 400 Bad Request"))
			})

			It("should error with a helpful message on 404 status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/releases/some-release-name"))
					w.WriteHeader(http.StatusNotFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Release("some-release-name")

				Expect(err).To(MatchError("release some-release-name could not be found"))
			})

			It("should error on an unsupported protocol", func() {
				client := bosh.NewClient(bosh.Config{
					URL:      "banana://example.com",
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Release("some-release-name")
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("should error on malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`&&%%%%%&%&%&%&%&%&%&%&`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Release("some-release-name")
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("should error on a malformed url", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "&&&&&%%%&%&%&%&%&",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Release("some-release-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

		})
	})

	Describe("Info", func() {
		It("fetches the director info", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/info"))
				Expect(r.Method).To(Equal("GET"))

				w.Write([]byte(`{"uuid":"some-director-uuid", "cpi":"some-cpi"}`))
			}))

			client := bosh.NewClient(bosh.Config{
				URL:                 server.URL,
				TaskPollingInterval: time.Nanosecond,
			})

			info, err := client.Info()

			Expect(err).NotTo(HaveOccurred())
			Expect(info).To(Equal(bosh.DirectorInfo{
				UUID: "some-director-uuid",
				CPI:  "some-cpi",
			}))
		})

		Context("failure cases", func() {
			It("should error on malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(`&&%%%%%&%&%&%&%&%&%&%&`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Info()

				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("should error on an unsupported protocol", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "banana://example.com",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Info()
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("errors on an unexpected status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadGateway)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.Info()
				Expect(err).To(MatchError("unexpected response 502 Bad Gateway"))
			})

		})
	})

	Context("DeleteDeployment", func() {
		It("deletes the given deployment", func() {
			callCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/deployments/some-deployment-name":
					Expect(r.Method).To(Equal("DELETE"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
				case "/tasks/1":
					Expect(r.Method).To(Equal("GET"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					if callCount == 3 {
						w.Write([]byte(`{"state": "done"}`))
					} else {
						w.Write([]byte(`{"state": "processing"}`))
					}
					callCount++
				default:
					Fail("could not match any URL endpoints")
				}
			}))

			client := bosh.NewClient(bosh.Config{
				URL:                 server.URL,
				Username:            "some-username",
				Password:            "some-password",
				TaskPollingInterval: time.Nanosecond,
			})

			err := client.DeleteDeployment("some-deployment-name")

			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(4))
		})

		Context("failure cases", func() {
			It("should error if the name is empty", func() {
				client := bosh.NewClient(bosh.Config{
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("")
				Expect(err).To(MatchError("a valid deployment name is required"))
			})

			It("should error on a non 302 redirect response", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusBadRequest)
					case "/tasks/1":
						Fail("should not have redirected to this task")
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")

				Expect(err).To(MatchError("unexpected response 400 Bad Request"))
			})

			It("should error on an errored task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "errored", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.Write([]byte(`
								{"state": "some-state"}
								{"error": {"code": 100, "message": "some-better-error-message"}}
							`))
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(errors.New("bosh task failed with an errored status \"some-better-error-message\"")))
			})

			It("should error on an error task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "error", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.Write([]byte(`
								{"state": "some-state"}
								{"error": {"code": 100, "message": "some-better-error-message"}}
							`))
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(errors.New("bosh task failed with an error status \"some-better-error-message\"")))
			})

			It("should error on a cancelled task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"state": "cancelled"}`))
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(errors.New("bosh task was cancelled")))
			})

			It("should error on a malformed redirect location", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", fmt.Sprintf("http://%s/%%%%%%%%%%%%%%", r.Host))
					w.WriteHeader(http.StatusFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error on a malformed url", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "&&&&&%%%&%&%&%&%&",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error on an unsupported protocol", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "banana://some-url",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("should error on malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
					w.Write([]byte(`&&%%%%%&%&%&%&%&%&%&%&`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				err := client.DeleteDeployment("some-deployment-name")

				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})

	Context("Deploy", func() {
		It("deploys the given manifest", func() {
			callCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/deployments":
					Expect(r.Method).To(Equal("POST"))
					Expect(r.Header.Get("Content-Type")).To(Equal("text/yaml"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					body, err := ioutil.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(body)).To(Equal("some-yaml"))

					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
				case "/tasks/1":

					Expect(r.Method).To(Equal("GET"))

					username, password, ok := r.BasicAuth()
					Expect(ok).To(BeTrue())
					Expect(username).To(Equal("some-username"))
					Expect(password).To(Equal("some-password"))

					if callCount == 3 {
						w.Write([]byte(`{"id": 1, "state": "done"}`))
					} else {
						w.Write([]byte(`{"id": 1, "state": "processing"}`))
					}
					callCount++
				default:
					Fail("could not match any URL endpoints")
				}
			}))

			client := bosh.NewClient(bosh.Config{
				URL:                 server.URL,
				Username:            "some-username",
				Password:            "some-password",
				TaskPollingInterval: time.Nanosecond,
			})

			taskId, err := client.Deploy([]byte("some-yaml"))

			Expect(err).NotTo(HaveOccurred())
			Expect(callCount).To(Equal(4))
			Expect(taskId).To(Equal(1))
		})

		Context("failure cases", func() {
			It("should error on a non 302 redirect response", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusBadRequest)
					case "/tasks/1":
						Fail("should not have redirected to this task")
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))

				Expect(err).To(MatchError("unexpected response 400 Bad Request"))
			})

			It("should error on an error task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "error", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.Write([]byte(`
								{"state": "some-state"}
								{"error": {"code": 100, "message": "some-better-error-message"}}
							`))
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(errors.New("bosh task failed with an error status \"some-better-error-message\"")))
			})

			It("return result error if events error fails", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "error", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.WriteHeader(http.StatusInternalServerError)
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(errors.New("failed to get full bosh task event log, bosh task failed with an error status \"some-error-message\"")))
			})

			It("should error on a errored task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "errored", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.WriteHeader(http.StatusInternalServerError)
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(errors.New("failed to get full bosh task event log, bosh task failed with an errored status \"some-error-message\"")))
			})

			It("should error on a errored task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"Id": 1, "state": "errored", "result": "some-error-message"}`))
					case "/tasks/1/output":
						if r.URL.RawQuery == "type=event" {
							w.Write([]byte(`
								{"state": "some-state"}
								{"error": {"code": 100, "message": "some-better-error-message"}}
							`))
						}
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(errors.New("bosh task failed with an errored status \"some-better-error-message\"")))
			})

			It("should error on a cancelled task status", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"state": "cancelled"}`))
					default:
						Fail("could not match any URL endpoints")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(errors.New("bosh task was cancelled")))
			})

			It("should error on a malformed redirect location", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", fmt.Sprintf("http://%s/%%%%%%%%%%%%%%", r.Host))
					w.WriteHeader(http.StatusFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error if there is no manifest", func() {
				client := bosh.NewClient(bosh.Config{
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte(""))
				Expect(err).To(MatchError("a valid manifest is required to deploy"))
			})

			It("should error on a malformed url", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "&&&&&%%%&%&%&%&%&",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error on an unsupported protocol", func() {
				client := bosh.NewClient(bosh.Config{
					URL:                 "banana://some-url",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("should error on malformed json", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
					w.Write([]byte(`&&%%%%%&%&%&%&%&%&%&%&`))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:                 server.URL,
					Username:            "some-username",
					Password:            "some-password",
					TaskPollingInterval: time.Nanosecond,
				})

				_, err := client.Deploy([]byte("some-yaml"))

				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})
		})
	})

	Describe("DeploymentVMs", func() {
		It("retrieves the list of deployment VMs given a deployment name", func() {
			var taskCallCount int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				switch r.URL.Path {
				case "/deployments/some-deployment-name/vms":
					Expect(r.URL.RawQuery).To(Equal("format=full"))
					host, _, err := net.SplitHostPort(r.Host)
					Expect(err).NotTo(HaveOccurred())

					location := &url.URL{
						Scheme: "http",
						Host:   host,
						Path:   "/tasks/1",
					}

					w.Header().Set("Location", location.String())
					w.WriteHeader(http.StatusFound)
				case "/tasks/1":
					w.WriteHeader(http.StatusAccepted)
					w.Write([]byte(`{"state":"done"}`))
					taskCallCount++
				case "/tasks/1/output":
					Expect(r.URL.RawQuery).To(Equal("type=result"))
					Expect(taskCallCount).NotTo(Equal(0))

					w.Write([]byte(`
						{"index": 0, "job_name": "consul_z1", "job_state":"some-state"}
						{"index": 0, "job_name": "etcd_z1", "job_state":"some-state"}
						{"index": 1, "job_name": "etcd_z1", "job_state":"some-other-state"}
						{"index": 2, "job_name": "etcd_z1", "job_state":"some-more-state"}
					`))
				default:
					Fail("unknown route")
				}
			}))

			client := bosh.NewClient(bosh.Config{
				URL:      server.URL,
				Username: "some-username",
				Password: "some-password",
			})

			vms, err := client.DeploymentVMs("some-deployment-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(vms).To(ConsistOf([]bosh.VM{
				{
					Index:   0,
					JobName: "consul_z1",
					State:   "some-state",
				},
				{
					Index:   0,
					JobName: "etcd_z1",
					State:   "some-state",
				},
				{
					Index:   1,
					JobName: "etcd_z1",
					State:   "some-other-state",
				},
				{
					Index:   2,
					JobName: "etcd_z1",
					State:   "some-more-state",
				},
			}))
		})

		Context("failure cases", func() {
			It("errors when the URL is malformed", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "http://%%%%%",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("errors when the protocol scheme is invalid", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "banana://example.com",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("unsupported protocol")))
			})

			It("errors when checking the task fails", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name/vms":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte("%%%"))
					default:
						Fail("unexpected route")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("should error on a non StatusFound status code", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/deployments/some-deployment-name/vms"))
					w.WriteHeader(http.StatusNotFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError("unexpected response 404 Not Found"))
			})

			It("errors when the redirect URL is malformed", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/deployments/some-deployment-name/vms"))
					w.Header().Set("Location", "http://%%%%%/tasks/1")
					w.WriteHeader(http.StatusFound)
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid URL escape")))
			})

			It("should error on malformed JSON", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
					w.WriteHeader(http.StatusFound)
					w.Write([]byte("%%%%%%\n%%%%%%%%%%%\n"))
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError(ContainSubstring("invalid character")))
			})

			It("should error on a bogus response body", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/deployments/some-deployment-name/vms":
						w.Header().Set("Location", fmt.Sprintf("http://%s/tasks/1", r.Host))
						w.WriteHeader(http.StatusFound)
					case "/tasks/1":
						w.Write([]byte(`{"state": "done"}`))
					case "/tasks/1/output":
						w.Write([]byte(""))
					default:
						Fail("unexpected route")
					}
				}))

				client := bosh.NewClient(bosh.Config{
					URL:      server.URL,
					Username: "some-username",
					Password: "some-password",
				})

				bosh.SetBodyReader(func(io.Reader) ([]byte, error) {
					return nil, errors.New("a bad read happened")
				})
				_, err := client.DeploymentVMs("some-deployment-name")
				Expect(err).To(MatchError("a bad read happened"))
			})
		})
	})

	Describe("ResolveManifestVersions", func() {
		It("resolves the latest versions of releases", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				switch r.URL.Path {
				case "/releases/consats":
					Expect(r.Method).To(Equal("GET"))
					w.Write([]byte(`{"versions":["2.0.0","3.0.0","4.0.0"]}`))
				case "/stemcells":
					Expect(r.Method).To(Equal("GET"))
					w.Write([]byte(`[
					{"name": "some-stemcell-name","version": "1.0.0"},
					{"name": "some-stemcell-name","version": "2.0.0"},
					{"name": "some-other-stemcell-name","version": "100.0.0"}
				]`))
				default:
					Fail("unexpected route")
				}
			}))

			client := bosh.NewClient(bosh.Config{
				URL:                 server.URL,
				Username:            "some-username",
				Password:            "some-password",
				TaskPollingInterval: time.Nanosecond,
			})

			manifest := `---
director_uuid: some-director-uuid
name: some-name
compilation: some-compilation-value
update: some-update-value
networks: some-networks-value
resource_pools:
- name: some-resource-pool-1
  network: some-network
  size: some-size
  cloud_properties: some-cloud-properties
  env: some-env
  stemcell:
    name: "some-stemcell-name"
    version: 1.0.0
- name: some-resource-pool-2
  network: some-network
  stemcell:
    name: "some-stemcell-name"
    version: latest
- name: some-resource-pool-3
  network: some-network
  stemcell:
    name: "some-other-stemcell-name"
    version: latest
jobs: some-jobs-value
properties: some-properties-value
releases:
- name: consul
  version: 2.0.0
- name: consats
  version: latest
`

			resolvedManifest, err := client.ResolveManifestVersions([]byte(manifest))
			Expect(err).NotTo(HaveOccurred())
			Expect(resolvedManifest).To(gomegamatchers.MatchYAML(`---
director_uuid: some-director-uuid
name: some-name
compilation: some-compilation-value
update: some-update-value
networks: some-networks-value
resource_pools:
- name: some-resource-pool-1
  network: some-network
  size: some-size
  cloud_properties: some-cloud-properties
  env: some-env
  stemcell:
    name: "some-stemcell-name"
    version: 1.0.0
- name: some-resource-pool-2
  network: some-network
  stemcell:
    name: "some-stemcell-name"
    version: 2.0.0
- name: some-resource-pool-3
  network: some-network
  stemcell:
    name: "some-other-stemcell-name"
    version: 100.0.0
jobs: some-jobs-value
properties: some-properties-value
releases:
- name: consul
  version: 2.0.0
- name: consats
  version: 4.0.0
`))
		})

		Context("failure cases", func() {
			Context("when the yaml is malformed", func() {
				It("returns an error", func() {
					client := bosh.NewClient(bosh.Config{})
					_, err := client.ResolveManifestVersions([]byte("%%%"))
					Expect(err).To(MatchError(ContainSubstring("yaml: ")))
				})
			})

			Context("when the stemcell API causes an error", func() {
				It("returns an error", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						Expect(r.URL.Path).To(Equal("/stemcells"))
						w.WriteHeader(http.StatusNotFound)
					}))

					client := bosh.NewClient(bosh.Config{
						URL:      server.URL,
						Username: "some-username",
						Password: "some-password",
					})
					manifest := `---
resource_pools:
- name: some-resource-pool
  network: some-network
  stemcell:
    name: "some-other-stemcell-name"
    version: latest
`

					_, err := client.ResolveManifestVersions([]byte(manifest))
					Expect(err).To(MatchError("stemcell some-other-stemcell-name could not be found"))
				})
			})

			Context("when the stemcell cannot resolve the latest", func() {
				It("returns an error", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						Expect(r.URL.Path).To(Equal("/stemcells"))
						Expect(r.Method).To(Equal("GET"))

						username, password, ok := r.BasicAuth()
						Expect(ok).To(BeTrue())
						Expect(username).To(Equal("some-username"))
						Expect(password).To(Equal("some-password"))

						w.Write([]byte(`[]`))

					}))

					client := bosh.NewClient(bosh.Config{
						URL:      server.URL,
						Username: "some-username",
						Password: "some-password",
					})
					manifest := `---
resource_pools:
- name: some-resource-pool
  network: some-network
  stemcell:
    name: "some-other-stemcell-name"
    version: latest
`

					_, err := client.ResolveManifestVersions([]byte(manifest))
					Expect(err).To(MatchError("no stemcell versions found, cannot get latest"))
				})
			})

			Context("when the release API causes an error", func() {
				It("returns an error", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						Expect(r.URL.Path).To(Equal("/releases/some-release-name"))
						w.WriteHeader(http.StatusNotFound)
					}))

					client := bosh.NewClient(bosh.Config{
						URL:      server.URL,
						Username: "some-username",
						Password: "some-password",
					})
					manifest := `---
releases:
- name: consul
  version: 2.0.0
- name: some-release-name
  version: latest
`

					_, err := client.ResolveManifestVersions([]byte(manifest))
					Expect(err).To(MatchError("release some-release-name could not be found"))
				})
			})
		})
	})

	Describe("UpdateCloudConfig", func() {
		It("updates cloud config", func() {
			testServerCallCount := 0
			cloudConfig := "some-cloud-config-yaml"

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				testServerCallCount++

				Expect(r.Method).To(Equal("POST"))
				Expect(r.URL.Path).To(Equal("/cloud_configs"))
				Expect(r.Header.Get("Content-Type")).To(Equal("text/yaml"))

				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				rawBody, err := ioutil.ReadAll(r.Body)

				Expect(err).NotTo(HaveOccurred())
				Expect(string(rawBody)).To(Equal(cloudConfig))

				w.WriteHeader(http.StatusCreated)
			}))

			client := bosh.NewClient(bosh.Config{
				URL:      testServer.URL,
				Username: "some-username",
				Password: "some-password",
			})

			err := client.UpdateCloudConfig([]byte(cloudConfig))

			Expect(err).NotTo(HaveOccurred())
			Expect(testServerCallCount).To(Equal(1))
		})

		Context("failure cases", func() {
			It("returns an error when request creation fails", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "%%%%%",
				})

				err := client.UpdateCloudConfig([]byte(""))

				Expect(err).To(MatchError(`parse %%%%%/cloud_configs: invalid URL escape "%%%"`))
			})

			It("returns an error when request fails", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "",
				})

				err := client.UpdateCloudConfig([]byte(""))

				Expect(err).To(MatchError(`unsupported protocol scheme ""`))
			})

			It("errors on an unexpected status code", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/cloud_configs"))

					w.WriteHeader(http.StatusTeapot)
				}))

				client := bosh.NewClient(bosh.Config{
					URL: testServer.URL,
				})

				err := client.UpdateCloudConfig([]byte(""))
				Expect(err).To(MatchError("unexpected response 418 I'm a teapot"))
			})
		})
	})

	Context("DownloadManifest", func() {
		It("downloads manifest", func() {
			testServerCallCount := 0

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				testServerCallCount++

				Expect(r.Method).To(Equal("GET"))
				Expect(r.URL.Path).To(Equal("/deployments/some-deployment-name"))

				username, password, ok := r.BasicAuth()
				Expect(ok).To(BeTrue())
				Expect(username).To(Equal("some-username"))
				Expect(password).To(Equal("some-password"))

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"manifest": "some-manifest-contents"}`))
			}))

			client := bosh.NewClient(bosh.Config{
				URL:      testServer.URL,
				Username: "some-username",
				Password: "some-password",
			})

			rawManifest, err := client.DownloadManifest("some-deployment-name")

			Expect(err).NotTo(HaveOccurred())
			Expect(testServerCallCount).To(Equal(1))
			Expect(rawManifest).To(Equal([]byte("some-manifest-contents")))
		})

		Context("failure cases", func() {
			It("returns an error when request is malformed", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "%%%%%",
				})

				_, err := client.DownloadManifest("some-deployment-name")
				Expect(err).To(MatchError(`parse %%%%%/deployments/some-deployment-name: invalid URL escape "%%%"`))
			})

			It("returns an error when the request fails", func() {
				client := bosh.NewClient(bosh.Config{
					URL: "",
				})

				_, err := client.DownloadManifest("")

				Expect(err).To(MatchError(`unsupported protocol scheme ""`))
			})

			It("errors on an unexpected status code", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/deployments/some-deployment-name"))

					w.WriteHeader(http.StatusTeapot)
				}))

				client := bosh.NewClient(bosh.Config{
					URL: testServer.URL,
				})

				_, err := client.DownloadManifest("some-deployment-name")
				Expect(err).To(MatchError("unexpected response 418 I'm a teapot"))
			})

			It("returns an error when server returns malformed json", func() {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					Expect(r.URL.Path).To(Equal("/deployments/some-deployment-name"))

					w.WriteHeader(http.StatusOK)
					w.Write([]byte("%%%%%%"))
				}))

				client := bosh.NewClient(bosh.Config{
					URL: testServer.URL,
				})

				_, err := client.DownloadManifest("some-deployment-name")
				Expect(err).To(MatchError("invalid character '%' looking for beginning of value"))
			})
		})
	})
})
