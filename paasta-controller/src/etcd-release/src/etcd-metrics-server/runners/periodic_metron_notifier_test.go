package runners_test

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/fakes"
	"github.com/cloudfoundry-incubator/etcd-metrics-server/runners"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// a bit of grace time for eventuallys
const aBit = 50 * time.Millisecond

var _ = Describe("PeriodicMetronNotifier", func() {
	var (
		leader   *ghttp.Server
		follower *ghttp.Server

		logger = lagertest.NewTestLogger("test")
		sender *fake.FakeMetricSender

		etcdURL        string
		reportInterval time.Duration

		metronNotifier ifrit.Process
		fakeGetter     *fakes.Getter
	)

	BeforeEach(func() {
		fakeGetter = &fakes.Getter{}
		leader = ghttp.NewServer()
		follower = ghttp.NewServer()

		keyHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Etcd-Index", "3")
			w.Header().Set("X-Raft-Index", "2")
			w.Header().Set("X-Raft-Term", "1")
		}

		leader.RouteToHandler("GET", "/v2/stats/leader", ghttp.RespondWith(200, fixtureLeaderStats))
		leader.RouteToHandler("GET", "/v2/stats/self", ghttp.RespondWith(200, fixtureSelfLeaderStats))
		leader.RouteToHandler("GET", "/v2/stats/store", ghttp.RespondWith(200, fixtureStoreStats))
		leader.RouteToHandler("GET", "/v2/keys/", keyHandler)

		follower.RouteToHandler("GET", "/v2/stats/leader", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, leader.URL(), 302)
		})

		follower.RouteToHandler("GET", "/v2/stats/self", ghttp.RespondWith(200, fixtureSelfFollowerStats))
		follower.RouteToHandler("GET", "/v2/stats/store", ghttp.RespondWith(200, fixtureStoreStats))
		follower.RouteToHandler("GET", "/v2/keys/", keyHandler)

		reportInterval = 100 * time.Millisecond
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)
	})

	JustBeforeEach(func() {
		metronNotifier = ifrit.Invoke(runners.NewPeriodicMetronNotifier(
			fakeGetter,
			etcdURL,
			logger,
			reportInterval,
		))
	})

	AfterEach(func() {
		metronNotifier.Signal(os.Interrupt)
		Eventually(metronNotifier.Wait(), 2*time.Second).Should(Receive())

		leader.Close()
		follower.Close()
	})

	Context("when the report interval elapses", func() {
		var metricEmitted = func(name string, value float64, unit string) {
			Eventually(func() fake.Metric {
				return sender.GetValue(name)
			}).Should(Equal(fake.Metric{
				Value: value,
				Unit:  unit,
			}), fmt.Sprintf("failed to get metric %s", name))
		}

		var metricNotEmitted = func(name string) {
			Consistently(func() fake.Metric {
				return sender.GetValue(name)
			}).Should(Equal(fake.Metric{}))
		}

		var itShouldEmitStoreResults = func() {
			metricEmitted("CompareAndDeleteFail", 0, runners.MetricUnit)
			metricEmitted("CompareAndDeleteSuccess", 4, runners.MetricUnit)
			metricEmitted("CompareAndSwapFail", 22, runners.MetricUnit)
			metricEmitted("CompareAndSwapSuccess", 50350, runners.MetricUnit)
			metricEmitted("CreateFail", 15252, runners.MetricUnit)
			metricEmitted("CreateSuccess", 18, runners.MetricUnit)
			metricEmitted("DeleteFail", 0, runners.MetricUnit)
			metricEmitted("DeleteSuccess", 0, runners.MetricUnit)
			metricEmitted("ExpireCount", 1, runners.MetricUnit)
			metricEmitted("GetsFail", 26705, runners.MetricUnit)
			metricEmitted("GetsSuccess", 10195, runners.MetricUnit)
			metricEmitted("SetsFail", 0, runners.MetricUnit)
			metricEmitted("SetsSuccess", 2540, runners.MetricUnit)
			metricEmitted("UpdateFail", 0, runners.MetricUnit)
			metricEmitted("UpdateSuccess", 0, runners.MetricUnit)
			metricEmitted("Watchers", 12, runners.MetricUnit)
			metricEmitted("EtcdIndex", 3, runners.MetricUnit)
			metricEmitted("RaftIndex", 2, runners.MetricUnit)
			metricEmitted("RaftTerm", 1, runners.MetricUnit)
		}

		Context("when the etcd node is a follower", func() {
			BeforeEach(func() {
				etcdURL = follower.URL()
			})

			It("should emit store statistics", func() {
				itShouldEmitStoreResults()
			})

			It("should emit self (follower) statistics", func() {
				metricEmitted("IsLeader", 0, runners.MetricUnit)
				metricEmitted("SentAppendRequests", 4321, runners.MetricUnit)
				metricEmitted("ReceivedAppendRequests", 1234, runners.MetricUnit)
				metricEmitted("ReceivingRequestRate", 2.0, runners.RequestsPerSecondUnit)
				metricEmitted("ReceivingBandwidthRate", 1.2, runners.BytesPerSecondUnit)
			})

			It("should not emit leader statistics", func() {
				metricNotEmitted("Followers")
				metricNotEmitted("Latency")
			})
		})

		Context("when the etcd node is a leader", func() {
			BeforeEach(func() {
				etcdURL = leader.URL()
			})

			It("should emit store statistics", func() {
				itShouldEmitStoreResults()
			})

			It("should emit self (leader) statistics", func() {
				metricEmitted("IsLeader", 1, runners.MetricUnit)
				metricEmitted("SentAppendRequests", 4321, runners.MetricUnit)
				metricEmitted("ReceivedAppendRequests", 1234, runners.MetricUnit)
				metricEmitted("SendingRequestRate", 5.0, runners.RequestsPerSecondUnit)
				metricEmitted("SendingBandwidthRate", 3.0, runners.BytesPerSecondUnit)
			})

			It("should emit leader statistics", func() {
				metricEmitted("Followers", 1, runners.MetricUnit)
				metricEmitted("Latency", 0.153507, runners.MetricUnit)

				// We wanted to test multiple followers and ensure that
				// multiple latencies were reported. But due to limitations of
				// the dropsonde protocol not accepting tags, the fake metrics
				// server cannot distinguish multiple latency metrics.
			})
		})
	})
})

var fixtureSelfFollowerStats = `
{
  "name": "node1",
				"id": "node1-id",
  "state": "StateFollower",

  "leaderInfo": {
	"leader": "node2-id",
					"uptime": "17h41m45.103057785s",
				  "startTime": "2015-02-13T01:28:26.657389108Z"
  },

  "recvAppendRequestCnt": 1234,
  "recvPkgRate": 2.0,
  "recvBandwidthRate": 1.2,

  "sendAppendRequestCnt": 4321
}`

var fixtureSelfLeaderStats = `
{
  "name": "node2",
				"id": "node2-id",
  "state": "StateLeader",

  "leaderInfo": {
	"leader": "node2-id",
					"uptime": "17h41m45.103057785s",
				  "startTime": "2015-02-13T01:28:26.657389108Z"
  },

  "recvAppendRequestCnt": 1234,

  "sendAppendRequestCnt": 4321,
  "sendPkgRate": 5.0,
  "sendBandwidthRate": 3.0
}`

var fixtureLeaderStats = `
{
  "leader": "node2-id",
  "followers": {
	"node1-id": {
	  "latency": {
		"current": 0.153507,
		"average": 0.14636559394884047,
		"standardDeviation": 0.15477392607571758,
		"minimum": 8.4e-05,
		"maximum": 6.78157
	  },
	  "counts": {
		"fail": 4,
		"success": 215000
	  }
	}
  }
}
`

var fixtureStoreStats = `
{
	"getsSuccess": 10195,
	"getsFail": 26705,
	"setsSuccess": 2540,
	"setsFail": 0,
	"deleteSuccess": 0,
	"deleteFail": 0,
	"updateSuccess": 0,
	"updateFail": 0,
	"createSuccess": 18,
	"createFail": 15252,
	"compareAndSwapSuccess": 50350,
	"compareAndSwapFail": 22,
	"compareAndDeleteSuccess": 4,
	"compareAndDeleteFail": 0,
	"expireCount": 1,
	"watchers": 12
}
`
