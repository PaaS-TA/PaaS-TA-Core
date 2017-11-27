package helpers_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Spammer", func() {
	var (
		kv      *fakeKV
		spammer *helpers.Spammer
		prefix  string
	)

	Context("Check", func() {
		BeforeEach(func() {
			prefix = fmt.Sprintf("some-prefix-%v", rand.Int())
			kv = newFakeKV()
			kv.AddressCall.Returns.Address = "http://some-address/consul"

			spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
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
			kv.GetCall.Returns.Error = fmt.Errorf("could not find key: %s-some-key-0", prefix)

			err := spammer.Check()
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("could not find key: %v-some-key-0", prefix))))
		})

		It("returns an error when a key doesn't match it's value", func() {
			Expect(kv.KeyVals).To(HaveKeyWithValue(fmt.Sprintf("%v-some-key-0", prefix), fmt.Sprintf("%v-some-value-0", prefix)))
			kv.KeyVals[fmt.Sprintf("%v-some-key-0", prefix)] = "banana"

			err := spammer.Check()
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("value for key \"%[1]v-some-key-0\" does not match: expected \"%[1]v-some-value-0\", got \"banana\"", prefix))))
		})

		Context("error tolerance", func() {
			BeforeEach(func() {
				kv = newFakeKV()
				kv.AddressCall.Returns.Address = "http://some-prefix-some-address/consul"
			})

			It("returns an error if no keys were written", func() {
				kv.SetCall.Returns.Error = errors.New("dial tcp some-prefix-some-address: getsockopt: connection refused")

				spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 1))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).To(MatchError(ContainSubstring("0 keys have been written")))
			})

			It("does not return an error when an underlying consul api client error occurs", func() {
				kv.SetCall.Stub = func(string, string) error {
					if kv.SetCall.CallCount.Value() < 10 {
						return errors.New("dial tcp some-prefix-some-address: getsockopt: connection refused")
					}
					return nil
				}

				spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 100))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error when an underlying proxy http error occurs", func() {
				kv.SetCall.Stub = func(string, string) error {
					if kv.SetCall.CallCount.Value() < 10 {
						return errors.New("unexpected status: 502 Bad Gateway")
					}
					return nil
				}

				spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 100))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when rpc errors are encountered", func() {
				It("resets rpc error count once a successful write occurs", func() {
					kv.SetCall.Stub = func(string, string) error {
						callCount := kv.SetCall.CallCount.Value()
						if callCount < helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT {
							return errors.New("rpc error")
						}

						if callCount < (helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT + 10) {
							return nil
						}

						if callCount < (2*helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT + 10) {
							return errors.New("rpc error")
						}

						return nil
					}

					spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
					spammer.Spam()

					// This eventually should make sure that a high number of keys have been written so that we do not fail in a separate key write percentage
					// error check.
					// On single core machines, the total key write amount would be low enough to create a high failure key write percentage
					Eventually(func() int {
						return kv.SetCall.CallCount.Value()
					}).Should(BeNumerically(">", helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT*10))

					spammer.Stop()

					err := spammer.Check()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when the number of sequential errors is less than the max", func() {
					It("does not return an error", func() {
						kv.SetCall.Stub = func(string, string) error {
							if kv.SetCall.CallCount.Value() < helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT {
								return errors.New("rpc error")
							}

							return nil
						}

						spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
						spammer.Spam()

						Eventually(func() int {
							return kv.SetCall.CallCount.Value()
						}).Should(BeNumerically(">", helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT*10))

						spammer.Stop()

						err := spammer.Check()
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("when the number of sequential errors is more than the max", func() {
					It("returns an error", func() {
						kv.SetCall.Returns.Error = errors.New("rpc error:")

						spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
						spammer.Spam()

						Eventually(func() int {
							return kv.SetCall.CallCount.Value()
						}).Should(BeNumerically(">", helpers.MAX_SUCCESSIVE_RPC_ERROR_COUNT))

						spammer.Stop()

						err := spammer.Check()
						Expect(err).To(MatchError(ContainSubstring("rpc error:")))
					})
				})
			})

			It("returns an error if the percentage of failed key writes is greater than the max threshold", func() {
				failureCalls := 800
				kv.SetCall.Stub = func(string, string) error {
					if kv.SetCall.CallCount.Value() <= failureCalls {
						return errors.New("something bad happened")
					}

					return nil
				}

				spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">=", failureCalls))

				spammer.Stop()

				err := spammer.Check()

				failureRate := (float32(failureCalls) / float32(kv.SetCall.CallCount.Value()) * 100)
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("too many keys failed to write: %.0f%% failure rate", failureRate))))
			})
		})

		Context("failure cases", func() {
			BeforeEach(func() {
				kv = newFakeKV()
				kv.AddressCall.Returns.Address = "http://some-prefix-some-address/consul"
			})

			It("returns all underlying communication errors during the spam", func() {
				kv.SetCall.Stub = func(string, string) error {
					callCount := kv.SetCall.CallCount.Value()
					switch {
					case callCount < 4:
						return errors.New("some error occurred")
					case callCount < 6:
						return errors.New("another error occurred")
					default:
						return nil
					}
				}

				spammer = helpers.NewSpammer(kv, time.Duration(0), prefix)
				spammer.Spam()

				Eventually(func() int {
					return kv.SetCall.CallCount.Value()
				}).Should(BeNumerically(">", 100))

				spammer.Stop()

				err := spammer.Check()
				Expect(err).To(Equal(helpers.ErrorSet{
					fmt.Sprintf("Error writing key \"%s-some-key-0\": some error occurred", prefix):    1,
					fmt.Sprintf("Error writing key \"%s-some-key-1\": some error occurred", prefix):    1,
					fmt.Sprintf("Error writing key \"%s-some-key-2\": some error occurred", prefix):    1,
					fmt.Sprintf("Error writing key \"%s-some-key-3\": another error occurred", prefix): 1,
					fmt.Sprintf("Error writing key \"%s-some-key-4\": another error occurred", prefix): 1,
				}))
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
