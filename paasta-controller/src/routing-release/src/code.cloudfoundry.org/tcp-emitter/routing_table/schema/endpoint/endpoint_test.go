package endpoint_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/tcp-emitter/routing_table/schema/endpoint"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingTableEntry", func() {
	var (
		source endpoint.ExternalEndpointInfos
	)
	BeforeEach(func() {
		source = endpoint.ExternalEndpointInfos{
			endpoint.NewExternalEndpointInfo("routing-group-1", 6000),
			endpoint.NewExternalEndpointInfo("routing-group-1", 6100),
		}
	})

	Context("Remove", func() {
		Context("when removing all the current elements", func() {
			It("returns an empty set", func() {
				deletingSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				Expect(resultSet).Should(Equal(endpoint.ExternalEndpointInfos{}))
			})
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				expectedSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing none of the current elements", func() {
			It("returns the same set", func() {
				deletingSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6200},
				}
				resultSet := source.Remove(deletingSet)
				expectedSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing an empty set", func() {
			It("returns the same set", func() {
				deletingSet := endpoint.ExternalEndpointInfos{}
				resultSet := source.Remove(deletingSet)
				expectedSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				Expect(resultSet).Should(Equal(expectedSet))
			})
		})

		Context("when removing from an empty set", func() {
			It("returns the same set", func() {
				source = endpoint.ExternalEndpointInfos{}
				deletingSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultSet := source.Remove(deletingSet)
				Expect(resultSet).Should(Equal(endpoint.ExternalEndpointInfos{}))
			})
		})
	})

	Context("RemoveExternalEndpoints", func() {
		var (
			sourceEntry     endpoint.RoutableEndpoints
			modificationTag *models.ModificationTag
			endpoints       map[endpoint.EndpointKey]endpoint.Endpoint
		)
		BeforeEach(func() {
			endpoints = map[endpoint.EndpointKey]endpoint.Endpoint{
				endpoint.NewEndpointKey("instance-guid-1", false): endpoint.NewEndpoint(
					"instance-guid-1", false, "some-ip-1", 62004, 5222, modificationTag),
				endpoint.NewEndpointKey("instance-guid-2", false): endpoint.NewEndpoint(
					"instance-guid-2", false, "some-ip-2", 62004, 5222, modificationTag),
			}
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			sourceEntry = endpoint.NewRoutableEndpoints(source, endpoints, "log-guid-1", modificationTag)
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6100},
				}
				resultEntry := sourceEntry.RemoveExternalEndpoints(deletingSet)
				expectedSet := endpoint.ExternalEndpointInfos{
					endpoint.ExternalEndpointInfo{RouterGroupGUID: "routing-group-1", Port: 6000},
				}
				expectedEntry := endpoint.NewRoutableEndpoints(expectedSet, endpoints, "log-guid-1", modificationTag)
				Expect(resultEntry).Should(Equal(expectedEntry))
			})
		})
	})

	Context("RoutingKeys Remove", func() {
		var (
			sourceRoutingKeys endpoint.RoutingKeys
		)

		BeforeEach(func() {
			sourceRoutingKeys = endpoint.RoutingKeys{
				endpoint.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
				endpoint.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
			}
		})

		Context("when removing all the current elements", func() {
			It("returns an empty set", func() {
				deletingRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					endpoint.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				Expect(resultSet).Should(Equal(endpoint.RoutingKeys{}))
			})
		})

		Context("when removing some of the current elements", func() {
			It("returns the remaining set", func() {
				deletingRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing none of the current elements", func() {
			It("returns the same set", func() {
				deletingRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-3", ContainerPort: 5002},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					endpoint.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing an empty set", func() {
			It("returns the same set", func() {
				deletingRoutingKeys := endpoint.RoutingKeys{}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				expectedRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-1", ContainerPort: 5000},
					endpoint.RoutingKey{ProcessGUID: "process-guid-2", ContainerPort: 5001},
				}
				Expect(resultSet).Should(Equal(expectedRoutingKeys))
			})
		})

		Context("when removing from an empty set", func() {
			It("returns the same set", func() {
				sourceRoutingKeys = endpoint.RoutingKeys{}
				deletingRoutingKeys := endpoint.RoutingKeys{
					endpoint.RoutingKey{ProcessGUID: "process-guid-3", ContainerPort: 5002},
				}
				resultSet := sourceRoutingKeys.Remove(deletingRoutingKeys)
				Expect(resultSet).Should(Equal(endpoint.RoutingKeys{}))
			})
		})
	})

})
