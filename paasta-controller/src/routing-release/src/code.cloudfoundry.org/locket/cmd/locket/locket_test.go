package main_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/cmd/locket/config"
	"code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/models"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Locket", func() {
	var (
		locketAddress string
		locketClient  models.LocketClient
		locketProcess ifrit.Process
		locketPort    uint16
		locketRunner  ifrit.Runner

		logger *lagertest.TestLogger

		configOverrides []func(*config.LocketConfig)
	)

	BeforeEach(func() {
		var err error

		locketPort, err = localip.LocalPort()
		Expect(err).NotTo(HaveOccurred())

		locketAddress = fmt.Sprintf("127.0.0.1:%d", locketPort)

		logger = lagertest.NewTestLogger("locket")

		configOverrides = []func(cfg *config.LocketConfig){
			func(cfg *config.LocketConfig) {
				cfg.ListenAddress = locketAddress
				cfg.ConsulCluster = consulRunner.ConsulCluster()
				cfg.DatabaseDriver = sqlRunner.DriverName()
				cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
			},
		}
	})

	JustBeforeEach(func() {
		var err error

		locketRunner = testrunner.NewLocketRunner(locketBinPath, configOverrides...)
		locketProcess = ginkgomon.Invoke(locketRunner)

		config := testrunner.ClientLocketConfig()
		config.LocketAddress = locketAddress
		locketClient, err = locket.NewClient(logger, config)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(locketProcess)
		sqlRunner.ResetTables(TruncateTableList)
	})

	Context("metrics", func() {
		var (
			testMetricsListener net.PacketConn
			testMetricsChan     chan *events.Envelope
		)

		BeforeEach(func() {
			testMetricsListener, _ = net.ListenPacket("udp", "127.0.0.1:0")
			testMetricsChan = make(chan *events.Envelope, 1)
			go func() {
				defer GinkgoRecover()
				for {
					buffer := make([]byte, 1024)
					n, _, err := testMetricsListener.ReadFrom(buffer)
					if err != nil {
						close(testMetricsChan)
						return
					}

					var envelope events.Envelope
					err = proto.Unmarshal(buffer[:n], &envelope)
					Expect(err).NotTo(HaveOccurred())
					testMetricsChan <- &envelope
				}
			}()
			port, err := strconv.Atoi(strings.TrimPrefix(testMetricsListener.LocalAddr().String(), "127.0.0.1:"))
			Expect(err).NotTo(HaveOccurred())

			configOverrides = append(configOverrides, func(cfg *config.LocketConfig) {
				cfg.DropsondePort = port
			})
		})

		It("emits metrics", func() {
			Eventually(testMetricsChan).Should(Receive())
		})
	})

	Context("debug address", func() {
		var debugAddress string

		BeforeEach(func() {
			port, err := localip.LocalPort()
			Expect(err).NotTo(HaveOccurred())

			debugAddress = fmt.Sprintf("127.0.0.1:%d", port)
			configOverrides = append(configOverrides, func(cfg *config.LocketConfig) {
				cfg.DebugAddress = debugAddress
			})
		})

		It("listens on the debug address specified", func() {
			_, err := net.Dial("tcp", debugAddress)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("ServiceRegistration", func() {
		It("registers itself with consul", func() {
			consulClient := consulRunner.NewClient()
			services, err := consulClient.Agent().Services()
			Expect(err).ToNot(HaveOccurred())

			Expect(services).To(HaveKeyWithValue("locket",
				&api.AgentService{
					Service: "locket",
					ID:      "locket",
					Port:    int(locketPort),
				}))
		})

		It("registers a TTL healthcheck", func() {
			consulClient := consulRunner.NewClient()
			checks, err := consulClient.Agent().Checks()
			Expect(err).ToNot(HaveOccurred())

			Expect(checks).To(HaveKeyWithValue("service:locket",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:locket",
					Name:        "Service 'locket' check",
					Status:      "passing",
					ServiceID:   "locket",
					ServiceName: "locket",
				}))
		})
	})

	Context("Lock", func() {
		It("locks the key with the corresponding value", func() {
			requestedResource := &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
			_, err := locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     requestedResource,
				TtlInSeconds: 10,
			})
			Expect(err).NotTo(HaveOccurred())

			resp, err := locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test"})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Resource).To(BeEquivalentTo(requestedResource))

			requestedResource = &models.Resource{Key: "test", Value: "test-data", Owner: "nima", Type: "lock"}
			_, err = locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     requestedResource,
				TtlInSeconds: 10,
			})
			Expect(err).To(HaveOccurred())
		})

		It("expires after a ttl", func() {
			requestedResource := &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
			_, err := locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     requestedResource,
				TtlInSeconds: 6,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test"})
				return err
			}, 10*time.Second).Should(HaveOccurred())
		})

		Context("when the lock server disappears unexpectedly", func() {
			It("still disappears after ~ the ttl", func() {
				requestedResource := &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
				_, err := locketClient.Lock(context.Background(), &models.LockRequest{
					Resource:     requestedResource,
					TtlInSeconds: 3,
				})
				Expect(err).NotTo(HaveOccurred())

				ginkgomon.Kill(locketProcess)

				// cannot reuse the runner otherwise a `exec: already started` error will occur
				locketRunner = testrunner.NewLocketRunner(locketBinPath, configOverrides...)
				locketProcess = ginkgomon.Invoke(locketRunner)

				Eventually(func() error {
					_, err := locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test"})
					return err
				}, 6*time.Second).Should(HaveOccurred())
			})
		})
	})

	Context("Release", func() {
		var requestedResource *models.Resource

		Context("when the lock does not exist", func() {
			It("throws an error releasing the lock", func() {
				requestedResource = &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
				_, err := locketClient.Release(context.Background(), &models.ReleaseRequest{Resource: requestedResource})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the lock exists", func() {
			JustBeforeEach(func() {
				requestedResource = &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
				_, err := locketClient.Lock(context.Background(), &models.LockRequest{Resource: requestedResource, TtlInSeconds: 10})
				Expect(err).NotTo(HaveOccurred())

				resp, err := locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test"})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Resource).To(BeEquivalentTo(requestedResource))
			})

			It("releases the lock", func() {
				_, err := locketClient.Release(context.Background(), &models.ReleaseRequest{Resource: requestedResource})
				Expect(err).NotTo(HaveOccurred())

				_, err = locketClient.Fetch(context.Background(), &models.FetchRequest{Key: "test"})
				Expect(err).To(HaveOccurred())
			})

			Context("when another process is the lock owner", func() {
				It("throws an error", func() {
					requestedResource = &models.Resource{Key: "test", Value: "test-data", Owner: "nima", Type: "lock"}
					_, err := locketClient.Release(context.Background(), &models.ReleaseRequest{Resource: requestedResource})
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Context("FetchAll", func() {
		var (
			resource1, resource2, resource3 *models.Resource
		)

		JustBeforeEach(func() {
			resource1 = &models.Resource{Key: "test", Value: "test-data", Owner: "jim", Type: "lock"}
			resource2 = &models.Resource{Key: "iamakey", Value: "test-data", Owner: "jim", Type: "presence"}
			resource3 = &models.Resource{Key: "newkey", Value: "test-data", Owner: "jim", Type: ""}

			_, err := locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     resource1,
				TtlInSeconds: 10,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     resource2,
				TtlInSeconds: 10,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = locketClient.Lock(context.Background(), &models.LockRequest{
				Resource:     resource3,
				TtlInSeconds: 10,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("fetches all the locks", func() {
			response, err := locketClient.FetchAll(context.Background(), &models.FetchAllRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Resources).To(ConsistOf(resource1, resource2, resource3))

			response, err = locketClient.FetchAll(context.Background(), &models.FetchAllRequest{Type: "presence"})
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Resources).To(ConsistOf(resource2))
		})
	})
})
