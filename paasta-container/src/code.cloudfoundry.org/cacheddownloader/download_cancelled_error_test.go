package cacheddownloader_test

import (
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DownloadCancelledError", func() {
	It("reports an error with source, duration and bytes", func() {
		e := cacheddownloader.NewDownloadCancelledError("here", 30*time.Second, 1)
		Expect(e.Error()).To(Equal("Download cancelled: source 'here', duration '30s', bytes '1'"))
	})

	Context("when no bytes have been read", func() {
		It("only reports source and duration", func() {
			e := cacheddownloader.NewDownloadCancelledError("here", 30*time.Second, cacheddownloader.NoBytesReceived)
			Expect(e.Error()).To(Equal("Download cancelled: source 'here', duration '30s'"))
		})
	})
})
