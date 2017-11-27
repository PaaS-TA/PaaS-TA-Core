package syslogchecker_test

import (
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/syslogchecker"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/syslogchecker/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/gomegamatchers"
)

const (
	SYSLOG_MANIFEST_PATH           = "/fake/go/path/src/github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/syslogchecker/assets/manifest.yml"
	APP_GUID                       = "37911525-ae36-46c5-aa4c-951551f192e6"
	runnerRunCallCountPerIteration = 12
)

type fakeGuidGenerator struct{}

func (fakeGuidGenerator) Generate() string {
	return "some-guid"
}

var _ = Describe("Checker", func() {
	var (
		logspinnerServer   *httptest.Server
		runner             *fakes.CFRunner
		checker            syslogchecker.Checker
		guidGenerator      fakeGuidGenerator
		mutex              sync.Mutex
		logSpinnerAppName  string
		messages           []string
		logOutput          []string
		syslogListenerPort int
	)

	var addMessage = func(message string) {
		mutex.Lock()
		defer mutex.Unlock()

		messages = append(messages, message)
	}

	var getMessages = func() []string {
		mutex.Lock()
		defer mutex.Unlock()

		copyOfMessages := make([]string, len(messages))
		copy(copyOfMessages, messages)

		return copyOfMessages
	}

	BeforeEach(func() {
		messages = []string{}
		guidGenerator = fakeGuidGenerator{}
		runner = &fakes.CFRunner{}

		syslogListenerPort = rand.Int()
		logSpinnerAppName = fmt.Sprintf("my-app-%v", rand.Int())
		logOutput = []string{
			fmt.Sprintf("2016-07-28T17:02:49.24-0700 [APP/PROC/WEB/0]      OUT ADDRESS: |127.0.0.1:%v|\n", syslogListenerPort),
			fmt.Sprintf("2016-07-28T17:02:49.24-0700 [APP/PROC/WEB/0]      %s", guidGenerator.Generate()),
		}

		os.Setenv("GOPATH", "/fake/go/path")

		runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
			if args[0] == "app" {
				return []byte(APP_GUID), nil
			}
			return []byte(strings.Join(logOutput, "")), nil
		}

		logspinnerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/log") && req.Method == "GET" {
				w.WriteHeader(http.StatusOK)
				parts := strings.Split(req.URL.Path, "/")
				message := parts[2]
				addMessage(message)

				return
			}
			w.WriteHeader(http.StatusTeapot)
		}))

		checker = syslogchecker.New("syslog-app", guidGenerator, 10*time.Millisecond, runner)
	})

	AfterEach(func() {
		logspinnerServer.Close()
	})

	Describe("Check", func() {
		It("returns okay and an error if the errors are below the threshold", func() {
			runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
				if args[0] == "push" && checker.GetIterationCount() < 2 {
					return nil, errors.New("an error occurred")
				}
				if args[0] == "unbind-service" && checker.GetIterationCount() < 2 {
					return nil, errors.New("an error occurred")
				}

				if args[0] == "app" {
					return []byte(APP_GUID), nil
				}
				return []byte(strings.Join(logOutput, "")), nil
			}

			checker.Start(logSpinnerAppName, logspinnerServer.URL)

			expectedIterations := 10
			expectedMinRunCommandCount := runnerRunCallCountPerIteration * (expectedIterations - 2)

			Eventually(func() int {
				return len(runner.RunCommand.Commands)
			}, "1s", "5ms").Should(BeNumerically(">", expectedMinRunCommandCount))

			err := checker.Stop()
			Expect(err).NotTo(HaveOccurred())

			ok, iterationCount, errPercent, errs := checker.Check()
			Expect(ok).To(BeTrue())

			errSet, ok := errs.(helpers.ErrorSet)
			Expect(ok).To(BeTrue())
			Expect(iterationCount).To(Equal(expectedIterations))
			Expect(errPercent).To(Equal(0.2))
			Expect(errSet).To(HaveLen(2))
			Expect(errSet).To(HaveKey("syslog drainer application push failed: "))
			Expect(errSet["syslog drainer application push failed: "]).To(Equal(2))
			Expect(errSet).To(HaveKey("could not unbind the logger from the application: "))
			Expect(errSet["could not unbind the logger from the application: "]).To(Equal(2))
		})

		It("returns not okay and an error if the errors surpass the threshold", func() {
			checker.Start(logSpinnerAppName, "not-a-real-app")

			expectedIterations := 10
			expectedMinRunCommandCount := runnerRunCallCountPerIteration * (expectedIterations - 1)

			Eventually(func() int {
				return len(runner.RunCommand.Commands)
			}, "1s", "5ms").Should(BeNumerically(">", expectedMinRunCommandCount))

			err := checker.Stop()
			Expect(err).NotTo(HaveOccurred())
			ok, _, _, errs := checker.Check()
			Expect(ok).To(BeFalse())
			Expect(errs).Should(HaveKey(`Get not-a-real-app/log/some-guid: unsupported protocol scheme ""`))
			Expect(errs).Should(HaveKey(fmt.Sprintf(`ran %d times, exceeded total error threshold of 0.2: 0.5`, expectedIterations)))
		})
	})

	Describe("Start", func() {
		It("streams logs from an app to the syslog listener", func() {
			sysLogAppName := "syslog-app-some-guid"

			checker.Start(logSpinnerAppName, logspinnerServer.URL)

			Eventually(func() int {
				return len(runner.RunCommand.Commands)
			}).Should(BeNumerically(">", 12))

			err := checker.Stop()
			Expect(err).NotTo(HaveOccurred())

			Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
				{"push", sysLogAppName, "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
				{"app", sysLogAppName, "--guid"},
				{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
				{"start", sysLogAppName},
				{"logs", sysLogAppName, "--recent"},
				{"cups", fmt.Sprintf("%s-service", sysLogAppName), "-l", fmt.Sprintf("syslog://127.0.0.1:%d", syslogListenerPort)},
				{"bind-service", logSpinnerAppName, fmt.Sprintf("%s-service", sysLogAppName)},
				{"restage", logSpinnerAppName},
				{"logs", sysLogAppName, "--recent"},
				{"unbind-service", logSpinnerAppName, fmt.Sprintf("%s-service", sysLogAppName)},
				{"delete-service", fmt.Sprintf("%s-service", sysLogAppName), "-f"},
				{"delete", sysLogAppName, "-f", "-r"},
			}))

			Expect(getMessages()).To(ContainElement("some-guid"))
		})

		Context("failure cases", func() {
			It("records an error when the request to the logger app fails", func() {
				logspinnerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if strings.HasPrefix(req.URL.Path, "/log") && req.Method == "GET" {
						w.WriteHeader(http.StatusNotFound)
						w.Write([]byte("app not found"))
						return
					}
					w.WriteHeader(http.StatusTeapot)
				}))

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 12))

				err := checker.Stop()
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = checker.Check()
				Expect(err).To(HaveKey("error sending get request to listener app: 404 - app not found"))
			})

			It("records an error when the syslog listener fails to validate it got the guid", func() {
				sysLogAppName := "syslog-app-some-guid"
				logOutput = []string{
					fmt.Sprintf("2016-07-28T17:02:49.24-0700 [APP/PROC/WEB/0]    OUT ADDRESS: |127.0.0.1:%v|\n", syslogListenerPort),
					fmt.Sprintf("2016-07-28T17:02:49.24-0700 [RTR/0]             log/%s", guidGenerator.Generate()),
				}

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 12))

				err := checker.Stop()
				Expect(err).NotTo(HaveOccurred())

				Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
					{"logs", sysLogAppName, "--recent"},
					{"unbind-service", logSpinnerAppName, fmt.Sprintf("%s-service", sysLogAppName)},
					{"delete-service", fmt.Sprintf("%s-service", sysLogAppName), "-f"},
					{"delete", sysLogAppName, "-f", "-r"},
				}))

				_, _, _, err = checker.Check()
				Expect(err).To(HaveKey("could not validate the guid on syslog"))
			})

			It("records the error when it could not get logs from syslog after app restage", func() {
				sysLogAppName := "syslog-app-some-guid"
				logfailed := false
				runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
					if args[0] == "logs" {
						if logfailed {
							return []byte("some error occurred"), &exec.ExitError{}
						}
						logfailed = true
					}
					if args[0] == "app" {
						return []byte(APP_GUID), nil
					}
					return []byte(strings.Join(logOutput, "")), nil
				}

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 12))

				err := checker.Stop()
				Expect(err).NotTo(HaveOccurred())

				Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
					{"push", sysLogAppName, "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
					{"app", sysLogAppName, "--guid"},
					{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
					{"start", sysLogAppName},
					{"logs", sysLogAppName, "--recent"},
					{"cups", fmt.Sprintf("%s-service", sysLogAppName), "-l", fmt.Sprintf("syslog://127.0.0.1:%d", syslogListenerPort)},
					{"bind-service", logSpinnerAppName, fmt.Sprintf("%s-service", sysLogAppName)},
					{"restage", logSpinnerAppName},
					{"logs", sysLogAppName, "--recent"},
				}))

				_, _, _, err = checker.Check()
				Expect(err).To(HaveKey("could not get the logs for syslog drainer app: some error occurred"))
			})

			DescribeTable("records and returns errors during start",
				func(command string, expectedCommands [][]string, unexpectedCommands []string, errMsg string) {
					logOutput = []string{
						"2016-07-28T17:02:49.24-0700 [APP/PROC/WEB/0]      OUT ADDRESS: |127.0.0.1:100|",
					}

					runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
						if args[0] == command {
							return []byte("some error occurred"), &exec.ExitError{}
						}
						if args[0] == "app" {
							return []byte(APP_GUID), nil
						}
						return []byte(strings.Join(logOutput, "")), nil
					}
					checker.Start("my-app-12345", logspinnerServer.URL)

					Eventually(func() int {
						return len(runner.RunCommand.Commands)
					}).Should(BeNumerically(">", 12))

					err := checker.Stop()
					Expect(err).NotTo(HaveOccurred())

					Expect(runner.RunCommand.Commands).To(gomegamatchers.ContainSequence(expectedCommands))

					Expect(runner.RunCommand.Commands).NotTo(ContainElement(unexpectedCommands))

					_, _, _, err = checker.Check()
					Expect(err).To(HaveKey(errMsg))
					Expect(getMessages()).To(HaveLen(0))
				},
				Entry("records an error when the syslog drainer app fails to push",
					"push",
					[][]string{{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"}},
					[]string{"app", "syslog-app-some-guid", "--guid"},
					"syslog drainer application push failed: some error occurred",
				),
				Entry("records an error when getting the app guid fails",
					"app",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
					},
					[]string{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
					`failed to get the guid for the app "syslog-app-some-guid": some error occurred`,
				),
				Entry("records an error when curl to enable diego fails",
					"curl",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
						{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
					},
					[]string{"start", "syslog-app-some-guid"},
					`failed to get enable diego for the app "syslog-app-some-guid": some error occurred`,
				),
				Entry("records an error when starting the syslog drainer app fails",
					"start",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
						{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
						{"start", "syslog-app-some-guid"},
					},
					[]string{"logs", "syslog-app-some-guid", "--recent"},
					"could not start the syslog-drainer app: some error occurred",
				),
				Entry("records an error when creating a user defined service fails",
					"cups",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
						{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
						{"start", "syslog-app-some-guid"},
						{"logs", "syslog-app-some-guid", "--recent"},
						{"cups", "syslog-app-some-guid-service", "-l", "syslog://127.0.0.1:100"},
					},
					[]string{"bind-service", "my-app-12345", "syslog-app-service"},
					"could not create the logger service: some error occurred",
				),
				Entry("records an error when creating a user defined service fails",
					"bind-service",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
						{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
						{"start", "syslog-app-some-guid"},
						{"logs", "syslog-app-some-guid", "--recent"},
						{"cups", "syslog-app-some-guid-service", "-l", "syslog://127.0.0.1:100"},
						{"bind-service", "my-app-12345", "syslog-app-some-guid-service"},
					},
					[]string{"restage", "my-app-12345"},
					"could not bind the logger to the application: some error occurred",
				),
				Entry("records an error when restaging the logspammer fails",
					"restage",
					[][]string{
						{"push", "syslog-app-some-guid", "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
						{"app", "syslog-app-some-guid", "--guid"},
						{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
						{"start", "syslog-app-some-guid"},
						{"logs", "syslog-app-some-guid", "--recent"},
						{"cups", "syslog-app-some-guid-service", "-l", "syslog://127.0.0.1:100"},
						{"bind-service", "my-app-12345", "syslog-app-some-guid-service"},
						{"restage", "my-app-12345"},
					},
					[]string{},
					"could not restage the app: some error occurred",
				),
			)

			It("records the error when IP address of the syslog drain app could not be obtained", func() {
				runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
					if args[0] == "logs" {
						return []byte("could not retrieve the logs"), &exec.ExitError{}
					}
					if args[0] == "app" {
						return []byte(APP_GUID), nil
					}
					return []byte{}, nil
				}
				sysLogAppName := "syslog-app-some-guid"

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 6))

				err := checker.Stop()
				Expect(err).NotTo(HaveOccurred())

				Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
					{"push", sysLogAppName, "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
					{"app", sysLogAppName, "--guid"},
					{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
					{"start", sysLogAppName},
					{"logs", sysLogAppName, "--recent"},
				}))

				_, _, _, err = checker.Check()
				Expect(err).To(HaveKey("could not retrieve the logs from syslog-drainer app: could not retrieve the logs"))
				Expect(getMessages()).To(HaveLen(0))
			})

			It("errors when syslog drainer logs does not contain a valid IP address", func() {
				logOutput = []string{}
				sysLogAppName := "syslog-app-some-guid"

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 6))

				Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
					{"push", sysLogAppName, "-f", SYSLOG_MANIFEST_PATH, "--no-start"},
					{"app", sysLogAppName, "--guid"},
					{"curl", fmt.Sprintf("/v2/apps/%s", APP_GUID), "-X", "PUT", "-d", `{"diego": true}`},
					{"start", sysLogAppName},
					{"logs", sysLogAppName, "--recent"},
				}))

				_, _, _, err := checker.Check()
				Expect(err).To(HaveKey("could not parse the IP address of syslog-drain app"))

			})

			It("performes all cleanup steps and records their errors", func() {
				sysLogAppName := "syslog-app-some-guid"
				runner.RunCommand.Receives.Stub = func(args ...string) ([]byte, error) {
					switch args[0] {
					case "delete":
						return []byte("application deletion failed"), &exec.ExitError{}
					case "delete-service":
						return []byte("delete-service failed"), &exec.ExitError{}
					case "unbind-service":
						return []byte("service not bound yet"), &exec.ExitError{}
					default:
						return []byte(fmt.Sprintf("2016-07-28T17:02:49.24-0700 [APP/PROC/WEB/0]      OUT ADDRESS: |127.0.0.1:%v|\n", syslogListenerPort)), nil
					}
				}

				checker.Start(logSpinnerAppName, logspinnerServer.URL)

				Eventually(func() int {
					return len(runner.RunCommand.Commands)
				}).Should(BeNumerically(">", 12))

				err := checker.Stop()
				Expect(err).NotTo(HaveOccurred())

				Eventually(runner.RunCommand.Commands).Should(gomegamatchers.ContainSequence([][]string{
					{"logs", sysLogAppName, "--recent"},
					{"unbind-service", logSpinnerAppName, fmt.Sprintf("%s-service", sysLogAppName)},
					{"delete-service", fmt.Sprintf("%s-service", sysLogAppName), "-f"},
					{"delete", sysLogAppName, "-f", "-r"},
				}))

				_, _, _, err = checker.Check()
				Expect(err).To(HaveKey("could not validate the guid on syslog"))
				Expect(err).To(HaveKey("could not delete the syslog drainer app: application deletion failed"))
				Expect(err).To(HaveKey("could not unbind the logger from the application: service not bound yet"))
				Expect(err).To(HaveKey("could not delete the service: delete-service failed"))
			})

		})
	})

	Describe("GetIterationCount", func() {
		It("returns the number of iterations the checker has gone through", func() {
			checker.Start(logSpinnerAppName, logspinnerServer.URL)

			Eventually(checker.GetIterationCount).Should(BeNumerically(">", 2))

			err := checker.Stop()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Stop", func() {
		It("no longer creates and bindes syslog drainers", func() {
			_ = checker.Start(logSpinnerAppName, logspinnerServer.URL)

			err := checker.Stop()
			Expect(err).NotTo(HaveOccurred())

			commandsRan := len(runner.RunCommand.Commands)

			Eventually(func() int {
				return len(runner.RunCommand.Commands)
			}, "100ms", "10ms").ShouldNot(BeNumerically(">", commandsRan))
		})
	})
})
