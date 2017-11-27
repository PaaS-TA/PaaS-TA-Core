package emitter_test

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/routing-api/fake_routing_api"
	apimodels "code.cloudfoundry.org/routing-api/models"
	fakeuaa "code.cloudfoundry.org/uaa-go-client/fakes"
	"code.cloudfoundry.org/uaa-go-client/schema"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("RoutingAPIEmitter", func() {

	var (
		routingApiClient      *fake_routing_api.FakeClient
		uaaClient             *fakeuaa.FakeClient
		routingEvents         routingtable.TCPRouteMappings
		expectedRoutingEvents routingtable.TCPRouteMappings
		routingAPIEmitter     emitter.RoutingAPIEmitter
		logger                lager.Logger
		ttl                   int
	)

	BeforeEach(func() {
		routingApiClient = new(fake_routing_api.FakeClient)
		ttl = 60
		logger = lagertest.NewTestLogger("test")
		uaaClient = &fakeuaa.FakeClient{}
		routingAPIEmitter = emitter.NewRoutingAPIEmitter(logger, routingApiClient, uaaClient, ttl)

		routingEvents = routingtable.TCPRouteMappings{
			Registrations: []apimodels.TcpRouteMapping{apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, 0)},
		}

		expectedRoutingEvents = routingtable.TCPRouteMappings{
			Registrations: []apimodels.TcpRouteMapping{apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, int(ttl))},
		}

		token := &schema.Token{
			AccessToken: "accesstoken",
		}
		uaaClient.FetchTokenReturns(token, nil)
	})

	It("fetches a client token from UAA", func() {
		err := routingAPIEmitter.Emit(routingEvents)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(uaaClient.FetchTokenCallCount()).To(Equal(1))
	})

	It("uses a cached token if available", func() {
		err := routingAPIEmitter.Emit(routingEvents)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(uaaClient.FetchTokenCallCount()).To(Equal(1))
		Expect(uaaClient.FetchTokenArgsForCall(0)).To(BeFalse())
	})

	It("authorizes the routing API call with its bearer token", func() {
		err := routingAPIEmitter.Emit(routingEvents)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(routingApiClient.SetTokenCallCount()).To(Equal(1))
	})

	Context("when UAA communication fails", func() {
		BeforeEach(func() {
			uaaClient.FetchTokenReturns(nil, errors.New("blam"))
		})

		It("returns an error and emits nothing", func() {
			err := routingAPIEmitter.Emit(routingEvents)
			Expect(err).Should(HaveOccurred())
			Expect(routingApiClient.UpsertTcpRouteMappingsCallCount()).To(Equal(0))
		})
	})

	Context("when valid routing events are provided", func() {

		Context("when router API returns no errors", func() {

			Context("and there are registration events", func() {
				It("emits valid upsert tcp routes and sets the ttl", func() {
					err := routingAPIEmitter.Emit(routingEvents)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(routingApiClient.UpsertTcpRouteMappingsCallCount()).To(Equal(1))
					mappingRequests := routingApiClient.UpsertTcpRouteMappingsArgsForCall(0)
					Expect(mappingRequests).To(ConsistOf(expectedRoutingEvents.Registrations))
				})
			})

			Context("and there are unregistration events", func() {
				BeforeEach(func() {
					routingEvents = routingtable.TCPRouteMappings{
						Unregistrations: []apimodels.TcpRouteMapping{apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, 0)},
					}
					expectedRoutingEvents = routingtable.TCPRouteMappings{
						Unregistrations: []apimodels.TcpRouteMapping{apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, 60)},
					}
				})

				It("emits valid delete tcp routes and sets the ttl", func() {
					err := routingAPIEmitter.Emit(routingEvents)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(routingApiClient.DeleteTcpRouteMappingsCallCount()).To(Equal(1))
					mappingRequests := routingApiClient.DeleteTcpRouteMappingsArgsForCall(0)
					Expect(mappingRequests).To(ConsistOf(expectedRoutingEvents.Unregistrations))
				})
			})
		})

		Context("when routing API Upsert returns an error", func() {
			BeforeEach(func() {
				routingApiClient.UpsertTcpRouteMappingsReturns(errors.New("unauthorized"))
			})

			It("retries once and logs the error", func() {
				err := routingAPIEmitter.Emit(routingEvents)
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("test.unable-to-upsert.*unauthorized"))
			})

			It("refreshes the cached UAA token", func() {
				err := routingAPIEmitter.Emit(routingEvents)
				Expect(err).To(HaveOccurred())

				Expect(uaaClient.FetchTokenCallCount()).To(Equal(2))
				Expect(uaaClient.FetchTokenArgsForCall(1)).To(BeTrue())
			})

			Context("when refreshing the cached token authorizes the emitter", func() {
				BeforeEach(func() {
					var count uint
					routingApiClient.UpsertTcpRouteMappingsStub = func([]apimodels.TcpRouteMapping) error {
						defer func() {
							count += 1
						}()
						if count > 0 {
							return nil
						}

						return errors.New("unauthorized")
					}
				})

				It("succeeds in emitting routes", func() {
					err := routingAPIEmitter.Emit(routingEvents)
					Expect(err).ToNot(HaveOccurred())

					Expect(uaaClient.FetchTokenCallCount()).To(Equal(2))
					Expect(routingApiClient.UpsertTcpRouteMappingsCallCount()).To(Equal(2))
				})
			})
		})

		Context("when routing API Delete returns an error", func() {
			BeforeEach(func() {
				routingApiClient.DeleteTcpRouteMappingsReturns(errors.New("unauthorized"))
				routingEvents = routingtable.TCPRouteMappings{
					Unregistrations: []apimodels.TcpRouteMapping{apimodels.NewTcpRouteMapping("123", 61000, "some-ip-1", 62003, int(ttl))},
				}
			})

			It("logs error", func() {
				err := routingAPIEmitter.Emit(routingEvents)
				Expect(err).To(HaveOccurred())
				Expect(logger).To(gbytes.Say("test.unable-to-delete.*unauthorized"))
			})

			It("refreshes the cached UAA token", func() {
				err := routingAPIEmitter.Emit(routingEvents)
				Expect(err).To(HaveOccurred())

				Expect(uaaClient.FetchTokenCallCount()).To(Equal(2))
				Expect(uaaClient.FetchTokenArgsForCall(1)).To(BeTrue())
			})

			Context("when refreshing the cached token authorizes the emitter", func() {
				BeforeEach(func() {
					var count uint
					routingApiClient.DeleteTcpRouteMappingsStub = func([]apimodels.TcpRouteMapping) error {
						defer func() {
							count += 1
						}()
						if count > 0 {
							return nil
						}

						return errors.New("unauthorized")
					}
				})

				It("succeeds in emitting routes", func() {
					err := routingAPIEmitter.Emit(routingEvents)
					Expect(err).ToNot(HaveOccurred())

					Expect(uaaClient.FetchTokenCallCount()).To(Equal(2))
					Expect(routingApiClient.DeleteTcpRouteMappingsCallCount()).To(Equal(2))
				})
			})
		})
	})
})
