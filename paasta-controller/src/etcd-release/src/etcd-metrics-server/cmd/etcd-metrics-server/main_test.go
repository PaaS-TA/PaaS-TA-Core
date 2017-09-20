package main_test

import (
	"net"
	"os"
	"os/exec"

	"github.com/cloudfoundry-incubator/etcd-metrics-server/runners"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Etcd Metrics Server", func() {
	const (
		CAFilePath   = "fixtures/etcd-ca.crt"
		CertFilePath = "fixtures/server.crt"
		KeyFilePath  = "fixtures/server.key"
	)

	type registration struct {
		Host        string   `json:host`
		Credentials []string `json:credentials`
	}

	etcdMetricsServerTest := func(sslConfig *etcdstorerunner.SSLConfig, args []string) {
		var etcdRunner *etcdstorerunner.ETCDClusterRunner
		var session *gexec.Session

		BeforeEach(func() {
			etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001, 1, sslConfig)
			etcdRunner.Start()
		})

		AfterEach(func() {
			etcdRunner.Stop()
			session.Kill().Wait()
		})

		It("starts the metron notifier correctly", func() {
			var err error
			udpConn, err := net.ListenPacket("udp4", "127.0.0.1:3456")
			Expect(err).ShouldNot(HaveOccurred())
			defer udpConn.Close()

			serverCmd := exec.Command(metricsServerPath, args...)
			serverCmd.Env = os.Environ()

			session, err = gexec.Start(serverCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			var nextEvent = func() *events.ValueMetric { return readNextEvent(udpConn) }
			var nextEventName = func() string { return *readNextEvent(udpConn).Name }

			Eventually(nextEvent, 15, 0.1).Should(Equal(&events.ValueMetric{
				Name:  proto.String("IsLeader"),
				Value: proto.Float64(1),
				Unit:  proto.String(runners.MetricUnit),
			}))

			Eventually(nextEventName, 15, 0.1).Should(Equal("RaftIndex"))
			Eventually(nextEventName, 15, 0.1).Should(Equal("RaftTerm"))
			Eventually(nextEventName, 15, 0.1).Should(Equal("RaftIndex"))
			Eventually(nextEventName, 15, 0.1).Should(Equal("EtcdIndex"))
		})
	}

	Context("with tls", func() {
		sslConfig := &etcdstorerunner.SSLConfig{
			CAFile:   CAFilePath,
			CertFile: CertFilePath,
			KeyFile:  KeyFilePath,
		}

		args := []string{
			"-jobName", "etcd-diego",
			"-port", "5678",
			"-etcdScheme", "https",
			"-etcdAddress", "127.0.0.1:5001",
			"-caCert", CAFilePath,
			"-cert", CertFilePath,
			"-key", KeyFilePath,
			"-metronAddress", "127.0.0.1:3456",
			"-reportInterval", "1s",
		}

		etcdMetricsServerTest(sslConfig, args)
	})

	Context("without tls", func() {
		args := []string{
			"-jobName", "etcd-diego",
			"-port", "5678",
			"-etcdAddress", "127.0.0.1:5001",
			"-metronAddress", "127.0.0.1:3456",
			"-reportInterval", "1s",
		}

		etcdMetricsServerTest(nil, args)
	})
})

func readNextEvent(udpConn net.PacketConn) *events.ValueMetric {
	bytes := make([]byte, 1024)
	n, _, err := udpConn.ReadFrom(bytes)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(n).Should(BeNumerically(">", 0))

	receivedBytes := bytes[:n]
	var event events.Envelope
	err = proto.Unmarshal(receivedBytes, &event)
	Expect(err).ShouldNot(HaveOccurred())
	return event.GetValueMetric()
}
