package leaderfinder_test

import (
	"errors"
	"net/url"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcd-proxy/leaderfinder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeFinder struct {
	FindCall struct {
		CallCount int
		Returns   struct {
			Address *url.URL
			Error   error
			Stub    func() (*url.URL, error)
		}
	}
}

func (f *fakeFinder) Find() (*url.URL, error) {
	f.FindCall.CallCount++
	if f.FindCall.Returns.Stub != nil {
		return f.FindCall.Returns.Stub()
	}

	return f.FindCall.Returns.Address, f.FindCall.Returns.Error
}

var _ = Describe("Manager", func() {
	var (
		finder     *fakeFinder
		defaultURL *url.URL
	)

	BeforeEach(func() {
		var err error

		finder = &fakeFinder{}
		defaultURL, err = url.Parse("https://etcd.service.cf.internal:4001")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("LeaderOrDefault", func() {
		It("returns the etcd cluster leader if it exists", func() {
			leaderURL, err := url.Parse("http://some.etcd.node:4001")
			Expect(err).NotTo(HaveOccurred())

			finder.FindCall.Returns.Address = leaderURL

			manager := leaderfinder.NewManager(defaultURL, finder)
			Eventually(func() *url.URL {
				return manager.LeaderOrDefault()
			}, "5s", "100ms").Should(Equal(leaderURL))
		})

		It("returns the default etcd url if the finder encounters an error", func() {
			finder.FindCall.Returns.Error = errors.New("could not find leader for some reason")

			manager := leaderfinder.NewManager(defaultURL, finder)
			Expect(manager.LeaderOrDefault()).To(Equal(defaultURL))
		})

		It("updates the current leader address in the background", func() {
			leaderURL, err := url.Parse("http://some.etcd.node:4001")
			Expect(err).NotTo(HaveOccurred())

			finder.FindCall.Returns.Stub = func() (*url.URL, error) {
				if finder.FindCall.CallCount < 5 {
					return nil, leaderfinder.LeaderNotFound
				}
				return leaderURL, nil
			}

			manager := leaderfinder.NewManager(defaultURL, finder)
			Expect(manager.LeaderOrDefault()).To(Equal(defaultURL))

			Eventually(func() *url.URL {
				return manager.LeaderOrDefault()
			}, "5s", "100ms").Should(Equal(leaderURL))
		})
	})
})
