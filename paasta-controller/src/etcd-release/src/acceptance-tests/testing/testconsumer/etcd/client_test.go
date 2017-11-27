package etcd_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/testconsumer/etcd"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/testconsumer/etcd/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var fakeClient *fakes.Client

	BeforeEach(func() {
		fakeClient = fakes.NewClient()
	})

	Describe("Get", func() {
		It("returns a value given a valid key", func() {
			fakeClient.GetCall.Returns.Value = func(key string) string {
				if key == "some-key" {
					return "some-value"
				}
				return ""
			}

			client := etcd.NewClient(fakeClient)
			value, err := client.Get("some-key")

			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(Equal("some-value"))
		})

		It("returns an error on an underlying etcd error", func() {
			fakeClient.GetCall.Returns.Error = errors.New("some etcd error")

			client := etcd.NewClient(fakeClient)
			_, err := client.Get("some-key")

			Expect(err).To(MatchError("some etcd error"))
		})
	})

	Describe("Set", func() {
		It("sets the value of a given key", func() {
			client := etcd.NewClient(fakeClient)
			err := client.Set("some-key", "some-value")

			Expect(err).NotTo(HaveOccurred())
			Expect(fakeClient.SetCall.Receives.Key).To(Equal("some-key"))
			Expect(fakeClient.SetCall.Receives.Value).To(Equal("some-value"))
			Expect(fakeClient.SetCall.Receives.TTL).To(Equal(uint64(6000)))
		})

		It("returns an error on an underlying etcd error", func() {
			fakeClient.SetCall.Returns.Error = errors.New("some etcd error")

			client := etcd.NewClient(fakeClient)
			err := client.Set("some-key", "some-value")
			Expect(err).To(MatchError("some etcd error"))
		})
	})

	Describe("Close", func() {
		It("calls close on goetcd client", func() {
			client := etcd.NewClient(fakeClient)
			client.Close()
			Expect(fakeClient.CloseCall.CallCount).To(Equal(1))
		})
	})
})
