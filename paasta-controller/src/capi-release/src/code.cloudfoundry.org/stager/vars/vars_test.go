package vars_test

import (
	"sort"
	"strings"

	"code.cloudfoundry.org/stager/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vars", func() {
	var sl vars.StringList

	BeforeEach(func() {
		sl = make(vars.StringList)
	})

	Describe("Get", func() {
		Context("when a single value is Set", func() {
			BeforeEach(func() {
				sl.Set("var1")
			})

			It("returns the single value", func() {
				result := sl.Get()
				resultSlice, ok := result.([]string)
				Expect(ok).To(BeTrue())
				Expect(resultSlice).To(Equal([]string{"var1"}))
			})
		})

		Context("when multiple unique values are Set", func() {
			BeforeEach(func() {
				sl.Set("var1")
				sl.Set("var2")
			})

			It("returns each value", func() {
				result := sl.Get()
				resultSlice, ok := result.([]string)
				Expect(ok).To(BeTrue())

				sort.Strings(resultSlice)
				Expect(resultSlice).To(Equal([]string{"var1", "var2"}))
			})
		})

		Context("when repeat values are Set", func() {
			BeforeEach(func() {
				sl.Set("var1")
				sl.Set("var2")
				sl.Set("var2")
				sl.Set("var3")
			})

			It("returns a unique list of the values", func() {
				result := sl.Get()
				resultSlice, ok := result.([]string)
				Expect(ok).To(BeTrue())

				sort.Strings(resultSlice)
				Expect(resultSlice).To(Equal([]string{"var1", "var2", "var3"}))
			})
		})
	})

	Describe("String", func() {
		It("returns the contents as a comma-separated string", func() {
			sl.Set("var1")
			result := sl.String()
			resultSlice := strings.Split(result, ",")
			Expect(resultSlice).To(Equal([]string{"var1"}))
		})
	})
})
