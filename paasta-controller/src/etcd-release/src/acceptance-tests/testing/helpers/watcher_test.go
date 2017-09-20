package helpers_test

import (
	"acceptance-tests/testing/helpers"
	"acceptance-tests/testing/helpers/fakes"
	"errors"
	"fmt"

	goetcd "github.com/coreos/go-etcd/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Watcher", func() {
	var (
		fakeWatcher *fakes.EtcdWatcher
		watcher     *helpers.Watcher
	)

	var pushWatcherResponse = func(key string, value string, modifiedIndex int) {
		watcher.Response <- &goetcd.Response{
			Node: &goetcd.Node{
				Key:           key,
				Value:         value,
				ModifiedIndex: uint64(modifiedIndex),
			},
		}
	}

	BeforeEach(func() {
		fakeWatcher = &fakes.EtcdWatcher{}
		fakeWatcher.WatchCall.Started = make(chan bool, 2)
		watcher = helpers.Watch(fakeWatcher, "/")
		<-fakeWatcher.WatchCall.Started
	})

	It("watches and records key changes", func() {
		go func() {
			for i := 1; i <= 4; i++ {
				pushWatcherResponse(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), i)
			}
			watcher.Stop <- true
		}()

		Eventually(watcher.IsStopped, "10s", "1s").Should(BeTrue())
		Expect(watcher.Data()).To(Equal(map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}))
	})

	It("starts the watcher up if it is prematurely closed", func() {
		pushWatcherResponse("key1", "value1", 1)

		fakeWatcher.WatchCall.Returns.Error = errors.New("EOF")
		fakeWatcher.WatchCall.Receives.Stop <- true
		<-fakeWatcher.WatchCall.Started

		pushWatcherResponse("key2", "value2", 2)
		pushWatcherResponse("key3", "value3", 3)

		fakeWatcher.WatchCall.Returns.Error = nil
		watcher.Stop <- true

		Eventually(watcher.IsStopped, "10s", "1s").Should(BeTrue())

		Expect(fakeWatcher.WatchCall.CallCount).To(Equal(2))
		Expect(fakeWatcher.WatchCall.Receives.WaitIndex).To(Equal(uint64(2)))
		Expect(watcher.Data()).To(Equal(map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}))
	})

	It("does not panic when the watcher has been closed", func() {
		watcher.Stop <- true

		Eventually(watcher.IsStopped, "2s", "1s").Should(BeTrue())
	})

	It("does not panic when the response is nil", func() {
		watcher.Response <- nil
		Expect(watcher.IsStopped()).To(BeFalse())
		Expect(watcher.Data()).To(Equal(map[string]string{}))
	})

	It("does not panic when the response node is nil", func() {
		watcher.Response <- &goetcd.Response{Node: nil}
		Expect(watcher.IsStopped()).To(BeFalse())
		Expect(watcher.Data()).To(Equal(map[string]string{}))
	})
})
