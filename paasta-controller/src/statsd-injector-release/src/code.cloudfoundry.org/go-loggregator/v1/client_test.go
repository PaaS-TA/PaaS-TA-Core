package v1_test

import (
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator/compatability"
	"code.cloudfoundry.org/go-loggregator/v1"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	lfake "github.com/cloudfoundry/dropsonde/log_sender/fake"
	mfake "github.com/cloudfoundry/dropsonde/metric_sender/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DropsondeClient", func() {
	var (
		client loggregator.IngressClient
	)

	JustBeforeEach(func() {
		var err error
		client, err = v1.NewClient()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when v2 api is disabled", func() {
		var (
			logSender    *lfake.FakeLogSender
			metricSender *mfake.FakeMetricSender
		)

		BeforeEach(func() {
			logSender = &lfake.FakeLogSender{}
			metricSender = mfake.NewFakeMetricSender()
			logs.Initialize(logSender)
			metrics.Initialize(metricSender, nil)
		})

		It("sends app logs", func() {
			client.SendAppLog("app-id", "message", "source-type", "source-instance")
			Expect(logSender.GetLogs()).To(ConsistOf(lfake.Log{AppId: "app-id", Message: "message",
				SourceType: "source-type", SourceInstance: "source-instance", MessageType: "OUT"}))
		})

		It("sends app error logs", func() {
			client.SendAppErrorLog("app-id", "message", "source-type", "source-instance")
			Expect(logSender.GetLogs()).To(ConsistOf(lfake.Log{AppId: "app-id", Message: "message",
				SourceType: "source-type", SourceInstance: "source-instance", MessageType: "ERR"}))
		})

		It("sends app metrics", func() {
			metric := events.ContainerMetric{
				ApplicationId: proto.String("app-id"),
			}
			client.SendAppMetrics(&metric)
			Expect(metricSender.Events()).To(ConsistOf(&metric))
		})

		It("sends component duration", func() {
			client.SendDuration("test-name", 1*time.Nanosecond)
			Expect(metricSender.HasValue("test-name")).To(BeTrue())
			Expect(metricSender.GetValue("test-name")).To(Equal(mfake.Metric{Value: 1, Unit: "nanos"}))
		})

		It("sends component data in MebiBytes", func() {
			client.SendMebiBytes("test-name", 100)
			Expect(metricSender.HasValue("test-name")).To(BeTrue())
			Expect(metricSender.GetValue("test-name")).To(Equal(mfake.Metric{Value: 100, Unit: "MiB"}))
		})

		It("sends component metric", func() {
			client.SendMetric("test-name", 1)
			Expect(metricSender.HasValue("test-name")).To(BeTrue())
			Expect(metricSender.GetValue("test-name")).To(Equal(mfake.Metric{Value: 1, Unit: "Metric"}))
		})

		It("sends component bytes/sec", func() {
			client.SendBytesPerSecond("test-name", 100.1)
			Expect(metricSender.HasValue("test-name")).To(BeTrue())
			Expect(metricSender.GetValue("test-name")).To(Equal(mfake.Metric{Value: 100.1, Unit: "B/s"}))
		})

		It("sends component req/sec", func() {
			client.SendRequestsPerSecond("test-name", 100.1)
			Expect(metricSender.HasValue("test-name")).To(BeTrue())
			Expect(metricSender.GetValue("test-name")).To(Equal(mfake.Metric{Value: 100.1, Unit: "Req/s"}))
		})
	})
})
