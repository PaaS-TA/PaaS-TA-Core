package fake_ccclient_test

import (
	"code.cloudfoundry.org/cc-uploader/ccclient"
	"code.cloudfoundry.org/cc-uploader/ccclient/fake_ccclient"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Fakes", func() {
	It("is an Uploader", func() {
		var _ ccclient.Uploader = &fake_ccclient.FakeUploader{}
	})

	It("is a Poller", func() {
		var _ ccclient.Poller = &fake_ccclient.FakePoller{}
	})
})
