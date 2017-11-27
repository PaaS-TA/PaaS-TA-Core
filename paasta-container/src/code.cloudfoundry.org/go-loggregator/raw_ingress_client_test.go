package loggregator_test

import (
	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-loggregator/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RawIngressClient", func() {
	var (
		client    *loggregator.RawIngressClient
		receivers chan loggregator_v2.Ingress_BatchSenderServer
		server    *testhelpers.TestIngressServer
	)

	BeforeEach(func() {
		var err error
		server, err = testhelpers.NewTestIngressServer(fixture("metron.crt"), fixture("metron.key"), fixture("CA.crt"))
		Expect(err).NotTo(HaveOccurred())

		err = server.Start()
		Expect(err).NotTo(HaveOccurred())
		receivers = server.Receivers()

		tlsConfig, err := loggregator.NewIngressTLSConfig(
			fixture("CA.crt"),
			fixture("client.crt"),
			fixture("client.key"),
		)
		Expect(err).NotTo(HaveOccurred())

		client, err = loggregator.NewRawIngressClient(
			server.Addr(),
			tlsConfig,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Stop()
	})

	It("reconnects when the server goes away and comes back", func() {
		client.Emit([]*loggregator_v2.Envelope{
			{
				Timestamp: 1,
			},
			{
				Timestamp: 2,
			},
		})

		envBatch, err := getBatch(receivers)
		Expect(err).NotTo(HaveOccurred())
		Expect(envBatch.Batch).To(HaveLen(2))

		server.Stop()
		Eventually(server.Start).Should(Succeed())

		Consistently(receivers).Should(BeEmpty())

		for i := 0; i < 200; i++ {
			client.Emit([]*loggregator_v2.Envelope{
				{
					Timestamp: 3,
				},
			})
		}

		envBatch, err = getBatch(receivers)
		Expect(err).NotTo(HaveOccurred())
		Expect(envBatch.Batch).ToNot(BeEmpty())
	})
})
