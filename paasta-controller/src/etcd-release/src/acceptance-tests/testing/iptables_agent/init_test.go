package main_test

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIPTablesAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "testing/iptables_agent")
}

var (
	pathToIPTablesAgent string
)

var _ = BeforeSuite(func() {
	var err error
	pathToIPTablesAgent, err = gexec.Build("acceptance-tests/testing/iptables_agent")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func openPort() (string, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, nil
}

func waitForServerToStart(port string) {
	timer := time.After(0 * time.Second)
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			panic("Failed to boot!")
		case <-timer:
			_, err := http.Get("http://localhost:" + port + "/kv/banana")
			if err == nil {
				return
			}

			timer = time.After(2 * time.Second)
		}
	}
}
