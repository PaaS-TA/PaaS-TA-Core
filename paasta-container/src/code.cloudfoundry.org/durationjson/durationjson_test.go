package durationjson_test

import (
	"time"

	"code.cloudfoundry.org/durationjson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Durationjson", func() {

	Context("Marshaling", func() {
		It("outputs time.Duration string values", func() {
			t := time.Duration(27 * time.Hour)
			d := durationjson.Duration(27 * time.Hour)

			b, err := d.MarshalJSON()
			Expect(err).NotTo(HaveOccurred())

			Expect(string(b)).To(Equal(`"` + t.String() + `"`))
		})
	})

	Context("Unmarshaling", func() {
		It("unmarshals golang-formatted time strings", func() {
			var d durationjson.Duration
			timeData := []byte("\"34s\"")

			err := d.UnmarshalJSON(timeData)
			Expect(err).NotTo(HaveOccurred())

			Expect(d).To(Equal(durationjson.Duration(34 * time.Second)))
		})

		Context("with invalid data", func() {
			It("returns an error", func() {
				var d durationjson.Duration
				timeData := []byte("{{")

				err := d.UnmarshalJSON(timeData)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with invalid time format", func() {
			It("returns an error", func() {
				var d durationjson.Duration
				timeData := []byte("\"34\"")

				err := d.UnmarshalJSON(timeData)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
