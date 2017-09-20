package main_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/dockerapplifecycle/builder"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Builder runner", func() {
	var (
		lifecycle        ifrit.Process
		builder          *main.Builder
		fakeDeamonRunner func(signals <-chan os.Signal, ready chan<- struct{}) error
	)

	BeforeEach(func() {
		// create a temporary file name to be used for the unix socket
		unixSocket, err := ioutil.TempFile(os.TempDir(), "")
		Expect(err).NotTo(HaveOccurred())
		// close the file so we don't get a "bind: address already in use" error
		unixSocket.Close()
		os.Remove(unixSocket.Name())

		fakeDeamonRunner = nil

		builder = &main.Builder{
			RepoName:               "ubuntu",
			Tag:                    "latest",
			OutputFilename:         "/tmp/result/result.json",
			DockerDaemonTimeout:    300 * time.Millisecond,
			CacheDockerImage:       true,
			DockerDaemonUnixSocket: unixSocket.Name(),
		}
	})

	Context("when a docker daemon runs", func() {
		JustBeforeEach(func() {
			lifecycle = ifrit.Background(grouper.NewParallel(os.Interrupt, grouper.Members{
				{"builder", ifrit.RunFunc(builder.Run)},
				{"fake_docker_daemon", ifrit.RunFunc(fakeDeamonRunner)},
			}))
		})

		AfterEach(func() {
			ginkgomon.Interrupt(lifecycle)
		})

		Context("when the daemon won't start", func() {
			BeforeEach(func() {
				fakeDeamonRunner = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					select {
					case signal := <-signals:
						return errors.New(signal.String())
					case <-time.After(1 * time.Second):
						// Daemon "crashes" after a while
					}
					return nil
				}
			})

			It("times out", func() {
				err := <-lifecycle.Wait()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out waiting for docker daemon to start"))
			})

			Context("and the process is interrupted", func() {
				JustBeforeEach(func() {
					lifecycle.Signal(os.Interrupt)
				})

				It("exists with error", func() {
					err := <-lifecycle.Wait()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake_docker_daemon exited with error: interrupt"))
					Expect(err.Error()).To(ContainSubstring("builder exited with error: interrupt"))
				})
			})
		})

		Context("when the daemon starts", func() {
			requestAddr := make(chan string, 1)
			var listener net.Listener

			BeforeEach(func() {
				var err error
				listener, err = net.Listen("unix", builder.DockerDaemonUnixSocket)
				Expect(err).NotTo(HaveOccurred())

				fakeDeamonRunner = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)

					go func() {
						defer GinkgoRecover()
						srvr := &http.Server{
							Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
								requestAddr <- request.RequestURI
							}),
							ReadTimeout:  time.Second,
							WriteTimeout: time.Second,
						}
						srvr.Serve(listener)
					}()

					return nil
				}
			})

			AfterEach(func() {
				Expect(listener.Close()).To(Succeed())
			})

			It("sends a ping request to the daemon", func() {
				Eventually(requestAddr).Should(Receive(Equal("/info")))
			})
		})
	})

	Describe("cached tags generation", func() {
		var (
			builder            main.Builder
			dockerRegistryIPs  []string
			dockerRegistryHost string
			dockerRegistryPort int
		)

		generateTag := func() (string, string) {
			image, err := builder.GenerateImageName()
			Expect(err).NotTo(HaveOccurred())

			parts := strings.Split(image, "/")
			Expect(parts).To(HaveLen(2))

			return parts[0], parts[1]
		}

		imageGeneration := func() {
			generatedImageNames := make(map[string]int)

			uniqueImageNames := func() bool {
				_, imageName := generateTag()
				generatedImageNames[imageName]++

				for key := range generatedImageNames {
					if generatedImageNames[key] != 1 {
						return false
					}
				}

				return true
			}

			It("generates different image names", func() {
				Consistently(uniqueImageNames).Should(BeTrue())
			})
		}

		BeforeEach(func() {
			builder = main.Builder{
				DockerRegistryIPs:  dockerRegistryIPs,
				DockerRegistryHost: dockerRegistryHost,
				DockerRegistryPort: dockerRegistryPort,
			}
		})

		Context("when there are several Docker Registry addresses", func() {
			dockerRegistryIPs = []string{"one", "two", "three", "four"}
			dockerRegistryHost = "docker-registry.service.cf.internal"
			dockerRegistryPort = 8080

			Describe("addresses", func() {
				hostOnly := func() string {
					address, _ := generateTag()
					return address
				}

				It("uses docker registry host and port", func() {
					Consistently(hostOnly).Should(Equal(fmt.Sprintf("%s:%d", dockerRegistryHost, dockerRegistryPort)))
				})
			})

			Describe("image names", imageGeneration)
		})

		Context("when there is a single Docker Registry address", func() {
			dockerRegistryIPs = []string{"one"}

			Describe("image names", imageGeneration)
		})
	})

})
