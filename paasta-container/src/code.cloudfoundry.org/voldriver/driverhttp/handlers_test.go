package driverhttp_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"time"
)

type RecordingCloseNotifier struct {
	*httptest.ResponseRecorder
	cn chan bool
}

func (rcn *RecordingCloseNotifier) CloseNotify() <-chan bool {
	return rcn.cn
}

func (rcn *RecordingCloseNotifier) SimulateClientCancel() {
	rcn.cn <- true
}

var _ = Describe("Volman Driver Handlers", func() {

	var testLogger = lagertest.NewTestLogger("HandlersTest")

	var ErrorResponse = func(res *RecordingCloseNotifier) voldriver.ErrorResponse {
		response := voldriver.ErrorResponse{}

		body, err := ioutil.ReadAll(res.Body)
		Expect(err).ToNot(HaveOccurred())

		err = json.Unmarshal(body, &response)
		Expect(err).ToNot(HaveOccurred())

		return response
	}

	Context("Activate", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := voldriver.Routes.FindRouteByName(voldriver.ActivateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when activate is successful", func() {
			JustBeforeEach(func() {
				driver.ActivateReturns(voldriver.ActivateResponse{Implements: []string{"VolumeDriver"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK with VolumeDriver info", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				activateResponse := voldriver.ActivateResponse{}

				body, err := ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &activateResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(activateResponse.Implements).Should(Equal([]string{"VolumeDriver"}))
			})
		})

		Context("when activate hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.ActivateStub = func(env voldriver.Env) voldriver.ActivateResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.ActivateResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.ActivateResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				activateResponse := voldriver.ActivateResponse{}

				body, err := ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &activateResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(activateResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("List", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := voldriver.Routes.FindRouteByName(voldriver.ListRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())

		})

		Context("when list is successful", func() {
			JustBeforeEach(func() {
				volume := voldriver.VolumeInfo{
					Name:       "fake-volume",
					Mountpoint: "fake-mountpoint",
				}
				listResponse := voldriver.ListResponse{
					Volumes: []voldriver.VolumeInfo{volume},
					Err:     "",
				}

				driver.ListReturns(listResponse)

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()
			})

			It("should respond 200 OK with the volume info", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				listResponse := voldriver.ListResponse{}

				body, err := ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &listResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(listResponse.Err).Should(BeEmpty())
				Expect(listResponse.Volumes[0].Name).Should(Equal("fake-volume"))
			})
		})

		Context("when the list hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.ListStub = func(env voldriver.Env) voldriver.ListResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.ListResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.ListResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				listResponse := voldriver.ListResponse{}

				body, err := ioutil.ReadAll(res.Body)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &listResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(listResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Mount", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		var ExpectMountPointToEqual = func(value string) voldriver.MountResponse {
			mountResponse := voldriver.MountResponse{}
			body, err := ioutil.ReadAll(res.Body)

			err = json.Unmarshal(body, &mountResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(mountResponse.Mountpoint).Should(Equal(value))
			return mountResponse
		}

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := voldriver.Routes.FindRouteByName(voldriver.MountRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)

			volumeId := "something"
			MountRequest := voldriver.MountRequest{
				Name: "some-volume",
				Opts: map[string]interface{}{"volume_id": volumeId},
			}
			mountJSONRequest, err := json.Marshal(MountRequest)
			Expect(err).NotTo(HaveOccurred())

			req, err = http.NewRequest("POST", path, bytes.NewReader(mountJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when mount is successful", func() {

			JustBeforeEach(func() {
				driver.MountReturns(voldriver.MountResponse{Mountpoint: "dummy_path"})

				wg.Add(1)
				testLogger.Info(fmt.Sprintf("%#v", res.Body))

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK with the mountpoint", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				ExpectMountPointToEqual("dummy_path")
			})
		})

		Context("when the mount hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.MountStub = func(env voldriver.Env, mountRequest voldriver.MountRequest) voldriver.MountResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.MountResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.MountResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				mountResponse := ExpectMountPointToEqual("")
				Expect(mountResponse.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Unmount", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			unmountRequest := voldriver.UnmountRequest{}
			unmountJSONRequest, err := json.Marshal(unmountRequest)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := voldriver.Routes.FindRouteByName(voldriver.UnmountRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(unmountJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when unmount is successful", func() {
			JustBeforeEach(func() {
				driver.UnmountReturns(voldriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()
				Expect(res.Code).To(Equal(200))
			})
		})

		Context("when the unmount hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.UnmountStub = func(env voldriver.Env, unmountRequest voldriver.UnmountRequest) voldriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.ErrorResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Get", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			getRequest := voldriver.GetRequest{}
			getJSONRequest, err := json.Marshal(getRequest)
			Expect(err).NotTo(HaveOccurred())

			By("then fake serving the response using the handler")
			route, found := voldriver.Routes.FindRouteByName(voldriver.GetRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(getJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when get is successful", func() {
			JustBeforeEach(func() {
				driver.GetReturns(voldriver.GetResponse{Volume: voldriver.VolumeInfo{Name: "some-volume", Mountpoint: "dummy_path"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should return 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				getResponse := voldriver.GetResponse{}
				body, err := ioutil.ReadAll(res.Body)
				err = json.Unmarshal(body, &getResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(getResponse.Volume.Name).Should(Equal("some-volume"))
				Expect(getResponse.Volume.Mountpoint).Should(Equal("dummy_path"))
			})
		})

		Context("when the get hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.GetStub = func(env voldriver.Env, getRequest voldriver.GetRequest) voldriver.GetResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.GetResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.GetResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Path", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			pathRequest := voldriver.PathRequest{Name: "some-volume"}
			pathJSONRequest, err := json.Marshal(pathRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := voldriver.Routes.FindRouteByName(voldriver.PathRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(pathJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when path is successful", func() {
			JustBeforeEach(func() {
				driver.PathReturns(voldriver.PathResponse{
					Mountpoint: "/some/mountpoint",
				})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				pathResponse := voldriver.PathResponse{}
				body, err := ioutil.ReadAll(res.Body)
				err = json.Unmarshal(body, &pathResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(pathResponse.Mountpoint).Should(Equal("/some/mountpoint"))
			})
		})

		Context("when the path hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.PathStub = func(env voldriver.Env, pathRequest voldriver.PathRequest) voldriver.PathResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.PathResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.PathResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Create", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			createRequest := voldriver.CreateRequest{Name: "some-volume"}
			createJSONRequest, err := json.Marshal(createRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := voldriver.Routes.FindRouteByName(voldriver.CreateRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(createJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when create is successful", func() {
			JustBeforeEach(func() {
				driver.CreateReturns(voldriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(BeEmpty())
			})
		})

		Context("when the create hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.CreateStub = func(env voldriver.Env, createRequest voldriver.CreateRequest) voldriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.ErrorResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Remove", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			removeRequest := voldriver.RemoveRequest{Name: "some-volume"}
			removeJSONRequest, err := json.Marshal(removeRequest)
			Expect(err).NotTo(HaveOccurred())

			route, found := voldriver.Routes.FindRouteByName(voldriver.RemoveRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader(removeJSONRequest))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when remove is successful", func() {
			JustBeforeEach(func() {
				driver.RemoveReturns(voldriver.ErrorResponse{})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(BeEmpty())
			})
		})

		Context("when the remove hangs and the client closes the connection", func() {
			JustBeforeEach(func() {
				driver.RemoveStub = func(env voldriver.Env, removeRequest voldriver.RemoveRequest) voldriver.ErrorResponse {
					ctx := env.Context()
					logger := env.Logger()
					for true {
						time.Sleep(time.Second * 1)

						select {
						case <-ctx.Done():
							logger.Error("from ctx", ctx.Err())
							return voldriver.ErrorResponse{Err: ctx.Err().Error()}
						}
					}
					return voldriver.ErrorResponse{}
				}
				wg.Add(2)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

				go func() {
					res.SimulateClientCancel()
					wg.Done()
				}()
			})

			It("should respond with context canceled", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				response := ErrorResponse(res)
				Expect(response.Err).Should(ContainSubstring("context canceled"))
			})
		})
	})

	Context("Capabilities", func() {
		var (
			err    error
			req    *http.Request
			res    *RecordingCloseNotifier
			driver *voldriverfakes.FakeDriver
			wg     sync.WaitGroup

			subject http.Handler
		)

		BeforeEach(func() {
			driver = &voldriverfakes.FakeDriver{}

			subject, err = driverhttp.NewHandler(testLogger, driver)
			Expect(err).NotTo(HaveOccurred())

			res = &RecordingCloseNotifier{
				ResponseRecorder: httptest.NewRecorder(),
				cn:               make(chan bool, 1),
			}

			route, found := voldriver.Routes.FindRouteByName(voldriver.CapabilitiesRoute)
			Expect(found).To(BeTrue())

			path := fmt.Sprintf("http://0.0.0.0%s", route.Path)
			req, err = http.NewRequest("POST", path, bytes.NewReader([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when capabilities is successful", func() {
			JustBeforeEach(func() {
				driver.CapabilitiesReturns(voldriver.CapabilitiesResponse{Capabilities: voldriver.CapabilityInfo{Scope: "global"}})

				wg.Add(1)

				go func() {
					subject.ServeHTTP(res, req)
					wg.Done()
				}()

			})

			It("should respond 200 OK", func() {
				wg.Wait()

				Expect(res.Code).To(Equal(200))

				capabilitiesResponse := voldriver.CapabilitiesResponse{}
				body, err := ioutil.ReadAll(res.Body)
				err = json.Unmarshal(body, &capabilitiesResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(capabilitiesResponse.Capabilities).Should(Equal(voldriver.CapabilityInfo{Scope: "global"}))
			})
		})
	})
})
