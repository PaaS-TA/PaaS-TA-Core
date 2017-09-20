package internal_test

import (
	"bytes"
	"fmt"

	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CfAuth", func() {
	var cmdStarter *fakeStarter
	var reporterOutput *bytes.Buffer

	BeforeEach(func() {
		cmdStarter = &fakeStarter{}
		reporterOutput = bytes.NewBuffer([]byte{})
		GinkgoWriter = reporterOutput
	})

	It("runs the cf auth command", func() {
		internal.CfAuth("user", "password", cmdStarter).Wait()
		Expect(cmdStarter.calledWith.executable).To(Equal("cf"))
		Expect(cmdStarter.calledWith.args).To(Equal([]string{"auth", "user", "password"}))
	})

	It("does not reveal the password", func() {
		internal.CfAuth("user", "password", cmdStarter).Wait()
		Expect(reporterOutput.String()).To(ContainSubstring("REDACTED"))
		Expect(reporterOutput.String()).NotTo(ContainSubstring("password"))
	})

	Context("when the starter returns error", func() {
		BeforeEach(func() {
			cmdStarter.toReturn.err = fmt.Errorf("something went wrong")
		})

		It("panics", func() {
			Expect(func() {
				internal.CfAuth("user", "password", cmdStarter)
			}).To(Panic())
		})
	})
})
