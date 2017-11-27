package helpers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/consulclient"
)

func SpamConsul(chan struct{}, *sync.WaitGroup, consulclient.HTTPKV) chan map[string]string {
	return make(chan map[string]string)
}

const (
	SUCCESSFUL_KEY_WRITE_THRESHOLD   = 0.75
	MAX_SUCCESSIVE_RPC_ERROR_COUNT   = 6
	MAX_CONNECTION_RESET_ERROR_COUNT = 3
	MAX_NO_KNOWN_CONSUL_ERROR_COUNT  = 3
)

type ErrorSet map[string]int

func (e ErrorSet) Error() string {
	message := "The following errors occurred:\n"
	for key, val := range e {
		message += fmt.Sprintf("  %s : %d\n", key, val)
	}
	return message
}

func (e ErrorSet) Add(err error) {
	e[err.Error()] = e[err.Error()] + 1
}

type kv interface {
	Address() string
	Set(key, value string) error
	Get(key string) (value string, err error)
}

type Spammer struct {
	kv                                        kv
	store                                     map[string]string
	testConsumerConnectionRefusedErrorMessage string
	testConsumerConnectionDroppedErrorMessage string
	done                                      chan struct{}
	wg                                        sync.WaitGroup
	intervalDuration                          time.Duration
	errors                                    ErrorSet
	keyWriteAttempts                          int
	prefix                                    string
}

func NewSpammer(kv kv, spamInterval time.Duration, prefix string) *Spammer {
	address := strings.TrimPrefix(strings.TrimSuffix(kv.Address(), "/consul"), "http://")
	linuxMessage := fmt.Sprintf("dial tcp %s: getsockopt: connection refused", address)
	windowsMessage := fmt.Sprintf("dial tcp %s: i/o timeout", address)
	return &Spammer{
		testConsumerConnectionRefusedErrorMessage: linuxMessage,
		testConsumerConnectionDroppedErrorMessage: windowsMessage,
		kv:               kv,
		store:            make(map[string]string),
		done:             make(chan struct{}),
		intervalDuration: spamInterval,
		errors:           ErrorSet{},
		prefix:           prefix,
	}
}

func (s *Spammer) Spam() {
	s.wg.Add(1)

	go func() {
		var counts struct {
			attempts              int
			rpcErrors             int
			connectionResetErrors int
			noKnownConsulErrors   int
		}
		for {
			select {
			case <-s.done:
				s.keyWriteAttempts = counts.attempts
				s.wg.Done()
				return
			case <-time.After(s.intervalDuration):
				counts.attempts++
				key := fmt.Sprintf("%s-some-key-%d", s.prefix, counts.attempts-1)
				value := fmt.Sprintf("%s-some-value-%d", s.prefix, counts.attempts-1)
				err := s.kv.Set(key, value)
				if err != nil {
					switch {
					case strings.Contains(err.Error(), "rpc error"):
						counts.rpcErrors++
						if counts.rpcErrors > MAX_SUCCESSIVE_RPC_ERROR_COUNT {
							s.errors.Add(fmt.Errorf("Error writing key %q: %s", key, err.Error()))
						}
					case strings.Contains(err.Error(), s.testConsumerConnectionDroppedErrorMessage):
						// failures to connect to the test consumer should not count as failed key writes
						// this typically happens when the test-consumer vm is rolled
						counts.attempts--
					case strings.Contains(err.Error(), s.testConsumerConnectionRefusedErrorMessage):
						// failures to connect to the test consumer should not count as failed key writes
						// this typically happens when the test-consumer vm is rolled
						counts.attempts--
					case strings.Contains(err.Error(), "unexpected status: 502 Bad Gateway"):
						// failure to connect to local consul agent via test consumer should be ignored
						// this typically happens when the local consul agent goes down before test-consumer
						counts.attempts--
					case strings.Contains(err.Error(), "http: proxy error"):
						counts.attempts--
					case strings.Contains(err.Error(), "read: connection reset by peer"):
						counts.connectionResetErrors++
						if counts.connectionResetErrors > MAX_CONNECTION_RESET_ERROR_COUNT {
							s.errors.Add(fmt.Errorf("Error writing key %q: %s", key, err.Error()))
						}
					case err.Error() == "unexpected status: 500 Internal Server Error  No known Consul servers":
						counts.noKnownConsulErrors++
						if counts.noKnownConsulErrors > MAX_NO_KNOWN_CONSUL_ERROR_COUNT {
							s.errors.Add(fmt.Errorf("Error writing key %q: %s", key, err.Error()))
						}
					default:
						s.errors.Add(fmt.Errorf("Error writing key %q: %s", key, err.Error()))
					}
				} else {
					counts.rpcErrors = 0
					s.store[key] = value
				}
			}
		}
	}()
}

func (s *Spammer) Stop() {
	close(s.done)
	s.wg.Wait()
}

func (s *Spammer) Check() error {
	if s.keyWriteAttempts == 0 {
		return errors.New("0 keys have been written")
	}

	successRate := float32(len(s.store)) / float32(s.keyWriteAttempts)
	if successRate < SUCCESSFUL_KEY_WRITE_THRESHOLD {
		failureRate := 1 - successRate
		s.errors.Add(fmt.Errorf("too many keys failed to write: %.0f%% failure rate", failureRate*100))
	}

	for k, v := range s.store {
		value, err := s.kv.Get(k)
		if err != nil {
			s.errors.Add(err)
			continue
		}

		if v != value {
			s.errors.Add(fmt.Errorf("value for key %q does not match: expected %q, got %q", k, v, value))
		}
	}

	if len(s.errors) > 0 {
		return s.errors
	}

	return nil
}
