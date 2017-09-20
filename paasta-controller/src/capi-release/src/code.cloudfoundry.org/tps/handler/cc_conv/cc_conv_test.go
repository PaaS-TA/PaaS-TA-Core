package cc_conv

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CC Conversion Tools", func() {
	var placementError string
	Describe("StateFor", func() {
		Context("without a placement error", func() {
			It("converts state from ActualLRPState to cc_messages LRPInstanceState", func() {
				Expect(StateFor(models.ActualLRPStateUnclaimed, placementError)).To(Equal(cc_messages.LRPInstanceStateStarting))
				Expect(StateFor(models.ActualLRPStateClaimed, placementError)).To(Equal(cc_messages.LRPInstanceStateStarting))
				Expect(StateFor(models.ActualLRPStateRunning, placementError)).To(Equal(cc_messages.LRPInstanceStateRunning))
				Expect(StateFor(models.ActualLRPStateCrashed, placementError)).To(Equal(cc_messages.LRPInstanceStateCrashed))
				Expect(StateFor("foobar", placementError)).To(Equal(cc_messages.LRPInstanceStateUnknown))
			})
		})

		Context("with a placement error", func() {
			BeforeEach(func() {
				placementError = "error"
			})

			It("converts state from ActualLRPState to cc_messages LRPInstanceState", func() {
				Expect(StateFor(models.ActualLRPStateUnclaimed, placementError)).To(Equal(cc_messages.LRPInstanceStateDown))
				Expect(StateFor(models.ActualLRPStateClaimed, placementError)).To(Equal(cc_messages.LRPInstanceStateStarting))
				Expect(StateFor(models.ActualLRPStateRunning, placementError)).To(Equal(cc_messages.LRPInstanceStateRunning))
				Expect(StateFor(models.ActualLRPStateCrashed, placementError)).To(Equal(cc_messages.LRPInstanceStateCrashed))
				Expect(StateFor("foobar", placementError)).To(Equal(cc_messages.LRPInstanceStateUnknown))
			})
		})
	})
})
