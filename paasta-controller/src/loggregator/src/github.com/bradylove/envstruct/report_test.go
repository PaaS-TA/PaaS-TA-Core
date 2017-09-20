package envstruct_test

import (
	"bytes"

	"github.com/bradylove/envstruct"

	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Report", func() {
	var (
		ts         SmallTestStruct
		outputText string
	)

	Describe("Report()", func() {
		BeforeEach(func() {
			for k, v := range baseEnvVars {
				os.Setenv(k, v)
			}

			err := envstruct.Load(&ts)
			Expect(err).ToNot(HaveOccurred())

			outputBuffer := bytes.NewBuffer(nil)
			envstruct.ReportWriter = outputBuffer

			err = envstruct.WriteReport(&ts)
			Expect(err).ToNot(HaveOccurred())

			outputText = string(outputBuffer.Bytes())
		})

		It("prints a report of the given envstruct struct", func() {
			Expect(outputText).To(Equal(expectedReportOutput))
		})
	})
})

const (
	expectedReportOutput = `FIELD NAME:       TYPE:     ENV:                REQUIRED:  VALUE:
HiddenThing       string    HIDDEN_THING        false      (OMITTED)
StringThing       string    STRING_THING        false      stringy thingy
BoolThing         bool      BOOL_THING          false      true
IntThing          int       INT_THING           false      100
URLThing          *url.URL  URL_THING           false      ` + urlOutput + `
StringSliceThing  []string  STRING_SLICE_THING  false      [one two three]
`
)
