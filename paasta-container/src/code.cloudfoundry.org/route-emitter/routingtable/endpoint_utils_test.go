package routingtable_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP Utils", func() {
	Describe("NewEndpointsFromActual", func() {
		Context("when actual is not evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}
				actualInfo := &routingtable.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							"2.2.2.2",
							models.NewPortMapping(11, 44),
							models.NewPortMapping(66, 99),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: tag,
					},
					Evacuating: false,
				}

				endpoints := routingtable.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
					routingtable.NewEndpoint("instance-guid", false, "1.1.1.1", "2.2.2.2", 11, 44, &tag),
					routingtable.NewEndpoint("instance-guid", false, "1.1.1.1", "2.2.2.2", 66, 99, &tag),
				}))
			})
		})

		Context("when actual is evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}

				actualInfo := &routingtable.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							"2.2.2.2",
							models.NewPortMapping(11, 44),
							models.NewPortMapping(66, 99),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: tag,
					},
					Evacuating: true,
				}

				endpoints := routingtable.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
					routingtable.NewEndpoint("instance-guid", true, "1.1.1.1", "2.2.2.2", 11, 44, &tag),
					routingtable.NewEndpoint("instance-guid", true, "1.1.1.1", "2.2.2.2", 66, 99, &tag),
				}))
			})
		})
	})

	Describe("NewRoutingKeysFromActual", func() {
		It("creates a list of keys for an actual LRP", func() {
			keys := routingtable.NewRoutingKeysFromActual(&routingtable.ActualLRPRoutingInfo{
				ActualLRP: &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						"2.2.2.2",
						models.NewPortMapping(11, 44),
						models.NewPortMapping(66, 99),
					),
					State: models.ActualLRPStateRunning,
				},
			})

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 44)))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 99)))
		})

		Context("when the actual lrp has no port mappings", func() {
			It("returns no keys", func() {
				keys := routingtable.NewRoutingKeysFromActual(&routingtable.ActualLRPRoutingInfo{
					ActualLRP: &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							"2.2.2.2",
						),
						State: models.ActualLRPStateRunning,
					},
				})

				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("NewRoutingKeysFromDesired", func() {
		It("creates a list of keys for an actual LRP", func() {
			routes := tcp_routes.TCPRoutes{
				{ExternalPort: 61000, ContainerPort: 8080},
				{ExternalPort: 61001, ContainerPort: 9090},
			}

			desired := (&models.DesiredLRP{
				Domain:      "tests",
				ProcessGuid: "process-guid",
				Ports:       []uint32{8080, 9090},
				Routes:      routes.RoutingInfo(),
				LogGuid:     "abc-guid",
			}).DesiredLRPSchedulingInfo()

			keys := routingtable.NewRoutingKeysFromDesired(&desired)

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 8080)))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 9090)))
		})

		Context("when the desired LRP does not define any container ports", func() {
			It("returns no keys", func() {
				routes := tcp_routes.TCPRoutes{}

				desired := (&models.DesiredLRP{
					Domain:      "tests",
					ProcessGuid: "process-guid",
					Routes:      routes.RoutingInfo(),
					LogGuid:     "abc-guid",
				}).DesiredLRPSchedulingInfo()

				keys := routingtable.NewRoutingKeysFromDesired(&desired)
				Expect(keys).To(HaveLen(0))
			})
		})
	})
})
