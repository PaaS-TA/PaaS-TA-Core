package invoker_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/goshims/execshim/exec_fake"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"

	"code.cloudfoundry.org/voldriver/invoker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RealInvoker", func() {
	var (
		subject    invoker.Invoker
		fakeCmd    *exec_fake.FakeCmd
		fakeExec   *exec_fake.FakeExec
		testLogger lager.Logger
		testCtx    context.Context
		testEnv    voldriver.Env
		cmd        = "some-fake-command"
		args       = []string{"fake-args-1"}
	)
	Context("when invoking an executable", func() {
		BeforeEach(func() {
			testLogger = lagertest.NewTestLogger("InvokerTest")
			testCtx = context.TODO()
			testEnv = driverhttp.NewHttpDriverEnv(testLogger, testCtx)
			fakeExec = new(exec_fake.FakeExec)
			fakeCmd = new(exec_fake.FakeCmd)
			fakeExec.CommandContextReturns(fakeCmd)
			subject = invoker.NewRealInvokerWithExec(fakeExec)
		})

		It("should successfully invoke cli", func() {
			_, err := subject.Invoke(testEnv, cmd, args)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when command fails", func() {
			BeforeEach(func() {
				fakeCmd.CombinedOutputReturns([]byte("an error occured"), fmt.Errorf("executing binary fails"))
			})

			It("should report an error", func() {
				_, err := subject.Invoke(testEnv, cmd, args)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("executing binary fails - details:\nan error occured"))
			})
			It("should return command output", func() {
				output, _ := subject.Invoke(testEnv, cmd, args)
				Expect(string(output)).To(Equal("an error occured"))
			})
		})
	})
})
