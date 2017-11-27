package component_tests_test

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"time"

	"google.golang.org/grpc/grpclog"

	v2 "github.com/cloudfoundry/statsd-injector/internal/plumbing/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("StatsdInjector", func() {
	var (
		consumerServer *MetronServer
		statsdAddr     string
		cleanup        func()
	)

	BeforeSuite(func() {
		grpclog.SetLogger(log.New(GinkgoWriter, "", 0))
	})

	BeforeEach(func() {
		var err error
		consumerServer, err = NewMetronServer()
		Expect(err).ToNot(HaveOccurred())

		statsdAddr, cleanup = startStatsdInjector(fmt.Sprint(consumerServer.Port()))
	})

	AfterEach(func() {
		consumerServer.Stop()
		cleanup()
	})

	It("emits envelopes produced from statsd", func() {
		connection, err := net.Dial("udp", statsdAddr)
		Expect(err).ToNot(HaveOccurred())
		defer connection.Close()
		statsdmsg := []byte("fake-origin.test.counter:23|g")

		go func() {
			for range time.Tick(time.Millisecond) {
				connection.Write(statsdmsg)
			}
		}()

		var receiver v2.Ingress_SenderServer
		Eventually(consumerServer.Metron.SenderInput.Arg0).Should(Receive(&receiver))

		f := func() bool {
			e, err := receiver.Recv()
			if err != nil {
				return false
			}

			if e.GetTags()["origin"].GetText() != "fake-origin" {
				return false
			}

			return e.GetGauge().GetMetrics()["test.counter"].GetValue() == 23
		}
		Eventually(f).Should(BeTrue())
	})
})

func startStatsdInjector(metronPort string) (statsdAddr string, cleanup func()) {
	path, err := gexec.Build("github.com/cloudfoundry/statsd-injector")
	Expect(err).ToNot(HaveOccurred())

	port := fmt.Sprint(testPort())

	cmd := exec.Command(path,
		"-statsd-port", port,
		"-metron-port", metronPort,
		"-ca", CAFilePath(),
		"-cert", StatsdCertPath(),
		"-key", StatsdKeyPath(),
	)
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	Expect(cmd.Start()).To(Succeed())

	return fmt.Sprintf("localhost:%s", port), func() {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

func testPort() int {
	add, _ := net.ResolveTCPAddr("tcp", ":0")
	l, _ := net.ListenTCP("tcp", add)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
