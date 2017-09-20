package acceptance_test

import (
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("check-a-record", func() {
	AfterEach(func() {
		dnsServer.DeregisterAllRecords()
	})

	Context("when no domain is provided", func() {
		It("prints usage and exits 1", func() {
			session := checkARecord([]string{})
			Eventually(session, time.Minute).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(Equal("usage: check-a-record <host>\n"))
		})
	})

	Context("when A records exist alongside MX and AAAA records", func() {
		It("exits 0 and prints only the A records", func() {
			dnsServer.RegisterARecord("domain-with-multiple-records", net.ParseIP("1.2.3.4"))
			dnsServer.RegisterARecord("domain-with-multiple-records", net.ParseIP("2.3.4.5"))
			dnsServer.RegisterAAAARecord("domain-with-multiple-records", net.ParseIP("2001:4860:0:2001::68"))
			dnsServer.RegisterMXRecord("domain-with-multiple-records", "some-mail-server.", 0)

			session := checkARecord([]string{"domain-with-multiple-records"})
			Eventually(session, time.Minute).Should(gexec.Exit(0))

			Expect(string(session.Out.Contents())).To(Equal("1.2.3.4\n2.3.4.5\n"))
		})
	})

	Context("when no A records exist", func() {
		It("exits 1 and prints an error when there are AAAA records", func() {
			dnsServer.RegisterAAAARecord("domain-with-aaaa-records", net.ParseIP("2001:4860:0:2001::68"))

			session := checkARecord([]string{"domain-with-aaaa-records"})
			Eventually(session, time.Minute).Should(gexec.Exit(1))

			Expect(string(session.Out.Contents())).To(Equal(""))
			Expect(string(session.Err.Contents())).To(Equal("No A records found\n"))
		})

		It("exits 1 and prints an error when there are non-AAAA records", func() {
			dnsServer.RegisterMXRecord("domain-with-mx-records", "some-mail-server.", 0)

			session := checkARecord([]string{"domain-with-mx-records"})
			Eventually(session, time.Minute).Should(gexec.Exit(1))

			Expect(string(session.Out.Contents())).To(Equal(""))
			Expect(string(session.Err.Contents())).To(Equal("No A records found (lookup domain-with-mx-records on 127.0.0.1:53: no such host)\n"))
		})
	})

	Context("when the domain does not exist at all", func() {
		It("exits 1 and prints an error", func() {
			session := checkARecord([]string{"nonexistent-domain"})
			Eventually(session, time.Minute).Should(gexec.Exit(1))

			Expect(string(session.Err.Contents())).To(Equal("No A records found (lookup nonexistent-domain on 127.0.0.1:53: no such host)\n"))
		})
	})
})
