package transformer_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/workpool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Transformer", func() {
	Describe("StepsRunner", func() {
		var (
			logger          lager.Logger
			optimusPrime    transformer.Transformer
			container       executor.Container
			logStreamer     log_streamer.LogStreamer
			gardenContainer *gardenfakes.FakeContainer
			clock           *fakeclock.FakeClock
		)

		BeforeEach(func() {
			gardenContainer = &gardenfakes.FakeContainer{}

			logger = lagertest.NewTestLogger("test-container-store")
			logStreamer = log_streamer.New("test", "test", 1)

			healthyMonitoringInterval := 1 * time.Millisecond
			unhealthyMonitoringInterval := 1 * time.Millisecond

			healthCheckWoorkPool, err := workpool.NewWorkPool(1)
			Expect(err).NotTo(HaveOccurred())

			clock = fakeclock.NewFakeClock(time.Now())

			optimusPrime = transformer.NewTransformer(
				nil, nil, nil, nil, nil, nil,
				os.TempDir(),
				false,
				healthyMonitoringInterval,
				unhealthyMonitoringInterval,
				healthCheckWoorkPool,
				clock,
				[]string{"/post-setup/path", "-x", "argument"},
				"jim",
			)

			container = executor.Container{
				RunInfo: executor.RunInfo{
					Setup: &models.Action{
						RunAction: &models.RunAction{
							Path: "/setup/path",
						},
					},
					Action: &models.Action{
						RunAction: &models.RunAction{
							Path: "/action/path",
						},
					},
					Monitor: &models.Action{
						RunAction: &models.RunAction{
							Path: "/monitor/path",
						},
					},
				},
			}
		})

		Context("when there is no run action", func() {
			BeforeEach(func() {
				container.Action = nil
			})

			It("returns an error", func() {
				_, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).To(HaveOccurred())
			})
		})

		It("returns a step encapsulating setup, post-setup, monitor, and action", func() {
			setupReceived := make(chan struct{})
			postSetupReceived := make(chan struct{})
			monitorProcess := &gardenfakes.FakeProcess{}
			gardenContainer.RunStub = func(processSpec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
				if processSpec.Path == "/setup/path" {
					setupReceived <- struct{}{}
				} else if processSpec.Path == "/post-setup/path" {
					postSetupReceived <- struct{}{}
				} else if processSpec.Path == "/monitor/path" {
					return monitorProcess, nil
				}
				return &gardenfakes.FakeProcess{}, nil
			}

			monitorProcess.WaitStub = func() (int, error) {
				if monitorProcess.WaitCallCount() == 1 {
					return 1, errors.New("boom")
				} else {
					return 0, nil
				}
			}

			runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
			Expect(err).NotTo(HaveOccurred())

			process := ifrit.Background(runner)

			Eventually(gardenContainer.RunCallCount).Should(Equal(1))
			processSpec, _ := gardenContainer.RunArgsForCall(0)
			Expect(processSpec.Path).To(Equal("/setup/path"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(1))

			<-setupReceived

			Eventually(gardenContainer.RunCallCount).Should(Equal(2))
			processSpec, _ = gardenContainer.RunArgsForCall(1)
			Expect(processSpec.Path).To(Equal("/post-setup/path"))
			Expect(processSpec.Args).To(Equal([]string{"-x", "argument"}))
			Expect(processSpec.User).To(Equal("jim"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(2))

			<-postSetupReceived

			Eventually(gardenContainer.RunCallCount).Should(Equal(3))
			processSpec, _ = gardenContainer.RunArgsForCall(2)
			Expect(processSpec.Path).To(Equal("/action/path"))
			Consistently(gardenContainer.RunCallCount).Should(Equal(3))

			Consistently(process.Ready()).ShouldNot(Receive())

			clock.Increment(1 * time.Second)
			Eventually(gardenContainer.RunCallCount).Should(Equal(4))
			processSpec, _ = gardenContainer.RunArgsForCall(3)
			Expect(processSpec.Path).To(Equal("/monitor/path"))
			Consistently(process.Ready()).ShouldNot(Receive())

			clock.Increment(1 * time.Second)
			Eventually(gardenContainer.RunCallCount).Should(Equal(5))
			processSpec, _ = gardenContainer.RunArgsForCall(4)
			Expect(processSpec.Path).To(Equal("/monitor/path"))
			Eventually(process.Ready()).Should(BeClosed())

			process.Signal(os.Interrupt)
			clock.Increment(1 * time.Second)
			Eventually(process.Wait()).Should(Receive(nil))
		})

		Context("when there is no setup", func() {
			BeforeEach(func() {
				container.Setup = nil
			})

			It("returns a codependent step for the action/monitor", func() {
				gardenContainer.RunReturns(&gardenfakes.FakeProcess{}, nil)

				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())

				process := ifrit.Background(runner)

				Eventually(gardenContainer.RunCallCount).Should(Equal(1))
				processSpec, _ := gardenContainer.RunArgsForCall(0)
				Expect(processSpec.Path).To(Equal("/action/path"))
				Consistently(gardenContainer.RunCallCount).Should(Equal(1))

				clock.Increment(1 * time.Second)
				Eventually(gardenContainer.RunCallCount).Should(Equal(2))
				processSpec, _ = gardenContainer.RunArgsForCall(1)
				Expect(processSpec.Path).To(Equal("/monitor/path"))
				Eventually(process.Ready()).Should(BeClosed())

				process.Signal(os.Interrupt)
				clock.Increment(1 * time.Second)
				Eventually(process.Wait()).Should(Receive(nil))
			})
		})

		Context("when there is no monitor", func() {
			BeforeEach(func() {
				container.Monitor = nil
			})

			It("does not run the monitor step and immediately says the healthcheck passed", func() {
				gardenContainer.RunReturns(&gardenfakes.FakeProcess{}, nil)

				runner, err := optimusPrime.StepsRunner(logger, container, gardenContainer, logStreamer)
				Expect(err).NotTo(HaveOccurred())

				process := ifrit.Background(runner)
				Eventually(process.Ready()).Should(BeClosed())

				Eventually(gardenContainer.RunCallCount).Should(Equal(3))
				processSpec, _ := gardenContainer.RunArgsForCall(2)
				Expect(processSpec.Path).To(Equal("/action/path"))
				Consistently(gardenContainer.RunCallCount).Should(Equal(3))
			})
		})
	})
})
