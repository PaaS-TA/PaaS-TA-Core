package helpers_test

import (
	"errors"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Spammer", func() {
	var (
		kv            *fakeKV
		spammer       *helpers.Spammer
		spammerPrefix string
	)

	Context("Check", func() {
		BeforeEach(func() {
			kv = newFakeKV()
			kv.AddressCall.Returns.Address = "http://some-address"

			spammerPrefix = "some-prefix"

			spammer = helpers.NewSpammer(kv, time.Duration(0), spammerPrefix)
			spammer.Spam()

			Eventually(func() int {
				return kv.SetCall.CallCount.Value()
			}).Should(BeNumerically(">", 1))

			spammer.Stop()
		})

		It("gets all the sets", func() {
			Expect(spammer.Check()).To(Succeed())
			Expect(kv.GetCall.CallCount).Should(Equal(kv.SetCall.CallCount.Value()))
		})

		It("returns an error when a key doesn't exist", func() {
			kv.GetCall.Returns.Error = errors.New("could not find key: some-prefix-some-key-0")

			err := spammer.Check()
			Expect(err).To(MatchError(ContainSubstring("could not find key: some-prefix-some-key-0")))
		})

		It("returns an error when a key doesn't match it's value", func() {
			Expect(kv.KeyVals).To(HaveKeyWithValue("some-prefix-some-key-0", "some-prefix-some-value-0"))
			kv.KeyVals["some-prefix-some-key-0"] = "banana"

			err := spammer.Check()
			Expect(err).To(MatchError(ContainSubstring("value for key \"some-prefix-some-key-0\" does not match: expected \"some-prefix-some-value-0\", got \"banana\"")))
		})

		Context("error tolerance", func() {
			BeforeEach(func() {
				kv = newFakeKV()
				kv.AddressCall.Returns.Address = "http://some-address"
			})

			It("returns an error if no keys were written", func() {
				kv.SetCall.Returns.Error = errors.New("dial tcp some-address: getsockopt: connection refused")

				spammer = helpers.NewSpammer(kv, time.Duration(0), "")
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 1))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).To(MatchError(ContainSubstring("0 keys have been written")))
			})

			It("does not return an error when an underlying etcd api client error occurs", func() {
				kv.SetCall.Stub = func(string, string) error {
					if kv.SetCall.CallCount.Value() < 10 {
						return errors.New("dial tcp some-address: getsockopt: connection refused")
					}
					return nil
				}

				spammer = helpers.NewSpammer(kv, time.Duration(0), "")
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 100))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

type atomicCount struct {
	value int
	sync.Mutex
}

func (c *atomicCount) inc() {
	c.Lock()
	defer c.Unlock()

	c.value++
}

func (c *atomicCount) Value() int {
	c.Lock()
	defer c.Unlock()

	return c.value
}

type fakeKV struct {
	KeyVals map[string]string

	AddressCall struct {
		Returns struct {
			Address string
		}
	}

	SetCall struct {
		CallCount *atomicCount
		Stub      func(string, string) error
		Receives  struct {
			Key   string
			Value string
		}
		Returns struct {
			Error error
		}
	}

	GetCall struct {
		CallCount int
		Receives  struct {
			Key string
		}
		Returns struct {
			Value string
			Error error
		}
	}
}

func newFakeKV() *fakeKV {
	kv := &fakeKV{KeyVals: map[string]string{}}
	kv.SetCall.CallCount = &atomicCount{}
	return kv
}

func (k *fakeKV) Set(key, value string) error {
	k.SetCall.CallCount.inc()
	k.SetCall.Receives.Key = key
	k.SetCall.Receives.Value = value

	k.KeyVals[key] = value

	if k.SetCall.Stub != nil {
		return k.SetCall.Stub(key, value)
	}

	return k.SetCall.Returns.Error
}

func (k *fakeKV) Get(key string) (string, error) {
	k.GetCall.CallCount++
	k.GetCall.Receives.Key = key

	return k.KeyVals[key], k.GetCall.Returns.Error
}

func (k *fakeKV) Address() string {
	return k.AddressCall.Returns.Address
}
