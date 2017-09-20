package helpers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type kv interface {
	Address() string
	Set(key, value string) error
	Get(key string) (value string, err error)
}

type Spammer struct {
	kv                                 kv
	mutex                              sync.Mutex
	store                              map[string]string
	testConsumerConnectionErrorMessage string
	done                               chan struct{}
	wg                                 sync.WaitGroup
	intervalDuration                   time.Duration
	errors                             ErrorSet
	keyWriteAttempts                   int
	prefix                             string
}

func NewSpammer(kv kv, spamInterval time.Duration, prefix string) *Spammer {
	address := strings.TrimPrefix(kv.Address(), "http://")
	message := fmt.Sprintf("dial tcp %s: getsockopt: connection refused", address)
	return &Spammer{
		testConsumerConnectionErrorMessage: message,
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
			attempts int
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
					case strings.Contains(err.Error(), s.testConsumerConnectionErrorMessage):
						// failures to connect to the test consumer should not count as failed key writes
						// this typically happens when the test-consumer vm is rolled
						counts.attempts--
					default:
						s.errors.Add(err)
					}
					continue
				}
				s.AddKVToStore(key, value)
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
	for k, v := range s.store {
		value, err := s.kv.Get(k)
		if err != nil {
			s.errors.Add(err)
			continue
		}

		if v != value {
			s.errors.Add(fmt.Errorf("value for key %q does not match: expected %q, got %q", k, v, value))
			continue
		}
	}

	if len(s.errors) > 0 {
		return s.errors
	}

	return nil
}

func (s *Spammer) ResetStore() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.store = map[string]string{}
}

func (s *Spammer) AddKVToStore(key, value string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.store[key] = value
}
