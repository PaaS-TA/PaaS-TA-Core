package leaderfinder

import (
	"net/url"
	"sync"
	"time"
)

type Manager struct {
	sync.Mutex
	address        *url.URL
	finder         finder
	defaultEtcdURL *url.URL
}

type finder interface {
	Find() (*url.URL, error)
}

func NewManager(defaultEtcdURL *url.URL, finder finder) *Manager {
	m := &Manager{
		finder:         finder,
		defaultEtcdURL: defaultEtcdURL,
		address:        defaultEtcdURL,
	}

	go func(m *Manager) {
		for {
			time.Sleep(500 * time.Millisecond)

			leaderURL, err := m.finder.Find()
			if err != nil {
				m.setAddress(m.defaultEtcdURL)
				continue
			}

			m.setAddress(leaderURL)
		}
	}(m)

	return m
}

func (m *Manager) setAddress(address *url.URL) {
	m.Lock()
	defer m.Unlock()

	m.address = address
}

func (m *Manager) LeaderOrDefault() *url.URL {
	m.Lock()
	defer m.Unlock()

	return m.address
}
