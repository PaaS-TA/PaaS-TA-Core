package http_routes

import (
	"io/ioutil"
	"os/exec"
	"time"

	"code.cloudfoundry.org/routing-acceptance-tests/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Registration", func() {
	var (
		expectedrtrVersion = "2.9"
		route              string
		routeJSON          string
	)

	Describe("HTTP Route", func() {
		var (
			eventsSession *Session
		)
		BeforeEach(func() {
			route = helpers.RandomName()
			routeJSON = `[{"route":"` + route + `","port":65340,"ip":"1.2.3.4","ttl":60}]`
			session, err := Start(exec.Command("rtr", "--version"), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session.Out, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(Say(expectedrtrVersion))
		})

		AfterEach(func() {
			if eventsSession != nil {
				eventsSession.Kill()
			}
		})

		It("can register, list, subscribe to sse and unregister routes", func() {
			args := []string{"events", "--http"}
			eventsSession = Rtr(args...)

			var eventsSessionLogs []byte
			Eventually(func() string {
				logAdd, err := ioutil.ReadAll(eventsSession.Out)
				Expect(err).ToNot(HaveOccurred())
				eventsSessionLogs = append(eventsSessionLogs, logAdd...)
				return string(eventsSessionLogs)
			}, 70*time.Second).Should(SatisfyAll(
				ContainSubstring(`"port":`),
				ContainSubstring(`"route":`),
				ContainSubstring(`"Action":"Upsert"`),
			))

			eventsSessionLogs = nil

			args = []string{"register", routeJSON}
			session := Rtr(args...)
			Eventually(session.Out, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(Say("Successfully registered routes"))

			Eventually(func() string {
				logAdd, err := ioutil.ReadAll(eventsSession.Out)
				Expect(err).ToNot(HaveOccurred())
				eventsSessionLogs = append(eventsSessionLogs, logAdd...)
				return string(eventsSessionLogs)
			}, 10*time.Second).Should(SatisfyAll(
				ContainSubstring(route),
				ContainSubstring(`"port":65340`),
				ContainSubstring(`"ip":"1.2.3.4"`),
				ContainSubstring(`"ttl":60`),
				ContainSubstring(`"Action":"Upsert"`),
			))

			args = []string{"list"}
			session = Rtr(args...)

			Eventually(session.Out, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(Say(route))

			args = []string{"unregister", routeJSON}
			session = Rtr(args...)

			Eventually(session.Out, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(Say("Successfully unregistered routes"))

			args = []string{"list"}
			session = Rtr(args...)

			Eventually(session.Out, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).ShouldNot(Say(route))
		})
	})
})
