package main_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("logspinner", func() {
	var (
		session *gexec.Session
		port    string
		url     string
	)

	BeforeEach(func() {
		var err error
		port, err = openPort()
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("PORT", port)

		url = fmt.Sprintf("http://localhost:%s", port)
	})

	AfterEach(func() {
		session.Terminate().Wait()
	})

	It("responds with 200 when server is hit on a port", func() {
		command := exec.Command(pathToLogspinner)

		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (int, error) {
			resp, err := http.Get(url)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			return resp.StatusCode, nil
		}, "10s", "1s").Should(Equal(http.StatusOK))
	})

	It("writes a log line when /log/{message} is hit", func() {
		command := exec.Command(pathToLogspinner)

		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		waitForServerToStart(port)

		resp, err := http.Get(fmt.Sprintf("%s/log/hello", url))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		Expect(session.Out.Contents()).To(ContainSubstring("hello"))
	})

	Context("failure cases", func() {
		It("returns a 400 when no message is passed to /log", func() {
			command := exec.Command(pathToLogspinner)

			var err error
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			waitForServerToStart(port)

			resp, err := http.Get(fmt.Sprintf("%s/log", url))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
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
			_, err := http.Get("http://localhost:" + port)
			if err == nil {
				return
			}

			timer = time.After(1 * time.Second)
		}
	}
}
