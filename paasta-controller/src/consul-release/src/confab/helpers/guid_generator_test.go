package helpers_test

import (
	"crypto/rand"
	"errors"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/fakes"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GenerateRandomUUID", func() {
	It("returns a randomly generated uuid", func() {
		uuid, err := helpers.GenerateRandomUUID(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(uuid).To(MatchRegexp(`\w{8}-\w{4}-\w{4}-\w{4}-\w{12}`))

		var uuids []string
		for i := 0; i < 10; i++ {
			uuid, err := helpers.GenerateRandomUUID(rand.Reader)
			Expect(err).NotTo(HaveOccurred())
			uuids = append(uuids, uuid)
		}
		Expect(HasUniqueValues(uuids)).To(BeTrue())
	})

	Context("failure cases", func() {
		It("returns an error when it cannot read from reader", func() {
			reader := &fakes.Reader{}
			reader.ReadCall.Returns.Error = errors.New("reader failed")

			_, err := helpers.GenerateRandomUUID(reader)
			Expect(err).To(MatchError("reader failed"))
		})
	})
})

func HasUniqueValues(values []string) bool {
	valueMap := make(map[string]struct{})

	for _, value := range values {
		valueMap[value] = struct{}{}
	}

	return len(valueMap) == len(values)
}
