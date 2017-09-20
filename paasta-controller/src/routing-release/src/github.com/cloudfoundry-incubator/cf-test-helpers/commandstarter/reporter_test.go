package commandstarter_test

import (
	"bytes"
	"io"
	"os/exec"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/commandstarter"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Reporter", func() {
	Describe("NewDefaultReporter", func() {
		var writers []io.Writer

		Context("when no writers are provided", func() {
			BeforeEach(func() {
				writers = []io.Writer{}
			})

			It("uses the GinkgoWriter", func() {
				reporter := commandstarter.NewDefaultReporter(writers...)
				writer := reporter.Writer
				Expect(writer).To(BeAssignableToTypeOf(GinkgoWriter))
			})
		})

		Context("when a single writer is provided", func() {
			BeforeEach(func() {
				writers = []io.Writer{
					&bytes.Buffer{},
				}
			})

			It("uses the provided writer", func() {
				reporter := commandstarter.NewDefaultReporter(writers...)
				Expect(reporter.Writer).To(Equal(writers[0]))
			})
		})

		Context("when there is more than one writer provided", func() {
			BeforeEach(func() {
				writers = []io.Writer{
					&bytes.Buffer{},
					&bytes.Buffer{},
				}
			})

			It("panics", func() {
				Expect(func() { commandstarter.NewDefaultReporter(writers...) }).To(Panic())
			})
		})
	})

	Describe("#Report", func() {
		var reporter *commandstarter.DefaultReporter
		var writer *gbytes.Buffer
		var t time.Time
		var timestampRegex string
		BeforeEach(func() {
			writer = gbytes.NewBuffer()
			reporter = commandstarter.NewDefaultReporter(writer)
			t = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
			timestampRegex = "\\[2009-11-10 23:00:00.00 \\(UTC\\)\\]>"
		})

		It("prints the timestamp and command in green", func() {
			cmd := exec.Command("executable", "arg1", "arg2")
			reporter.Report(t, cmd)

			lineStart := "^\n"
			greenStart := "\\x1b\\[32m"
			greenEnd := "\\x1b\\[0m"
			lineEnd := "\n$"
			Expect(writer).To(gbytes.Say("%s%s%s executable arg1 arg2 %s%s", lineStart, greenStart, timestampRegex, greenEnd, lineEnd))
		})

		Context("when NoColor is specified", func() {
			BeforeEach(func() {
				config.DefaultReporterConfig.NoColor = true
			})

			It("does not print color", func() {
				cmd := exec.Command("executable", "arg1", "arg2")
				reporter.Report(t, cmd)

				lineStart := "^\n"
				lineEnd := "\n$"
				Expect(writer).To(gbytes.Say("%s%s executable arg1 arg2 %s", lineStart, timestampRegex, lineEnd))

			})
		})
	})
})
