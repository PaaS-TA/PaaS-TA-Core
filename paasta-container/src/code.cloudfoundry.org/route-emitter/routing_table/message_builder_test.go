package routing_table_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routing_table"
	. "code.cloudfoundry.org/route-emitter/routing_table/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessagesToEmitBuilder", func() {
	var builder routing_table.MessagesToEmitBuilder
	var existingEntry *routing_table.RoutableEndpoints
	var newEntry *routing_table.RoutableEndpoints
	var messages routing_table.MessagesToEmit
	var domains models.DomainSet

	hostname1 := "foo.example.com"
	hostname2 := "bar.example.com"
	hostname3 := "baz.example.com"
	domain := "tests"

	currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
	endpoint1 := routing_table.Endpoint{InstanceGuid: "ig-1", Host: "1.1.1.1", Index: 0, Domain: domain, Port: 11, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
	endpoint2 := routing_table.Endpoint{InstanceGuid: "ig-2", Host: "2.2.2.2", Index: 1, Domain: domain, Port: 22, ContainerPort: 8080, Evacuating: false, ModificationTag: currentTag}
	freshDomains := models.NewDomainSet([]string{"tests"})
	noFreshDomains := models.NewDomainSet([]string{"foo"})

	BeforeEach(func() {
		builder = routing_table.MessagesToEmitBuilder{}
	})

	Describe("UnfreshRegistrations", func() {
		BeforeEach(func() {
			existingEntry = &routing_table.RoutableEndpoints{
				Routes: []routing_table.Route{
					routing_table.Route{Hostname: hostname1},
					routing_table.Route{Hostname: hostname2},
				},
				Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
			}
		})

		JustBeforeEach(func() {
			messages = builder.UnfreshRegistrations(existingEntry, domains)
		})

		Context("when domain is fresh", func() {
			BeforeEach(func() {
				domains = freshDomains
			})

			It("emits nothing", func() {
				Expect(messages).To(BeZero())
			})
		})

		Context("when domain is not fresh", func() {
			BeforeEach(func() {
				domains = noFreshDomains
			})

			It("does emits a registration", func() {
				expected := routing_table.MessagesToEmit{
					RegistrationMessages: []routing_table.RegistryMessage{
						routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
						routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
					},
				}
				Expect(messages).To(MatchMessagesToEmit(expected))
			})
		})
	})

	Describe("MergedRegistrations", func() {
		BeforeEach(func() {
			existingEntry = &routing_table.RoutableEndpoints{
				Routes: []routing_table.Route{
					routing_table.Route{Hostname: hostname1},
					routing_table.Route{Hostname: hostname2},
				},
				Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
			}
		})

		JustBeforeEach(func() {
			messages = builder.MergedRegistrations(existingEntry, newEntry, domains)
		})

		Context("when domain is fresh", func() {
			BeforeEach(func() {
				domains = freshDomains
			})

			Context("when reemitting the previous endpoints", func() {
				BeforeEach(func() {
					newEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname1},
							routing_table.Route{Hostname: hostname3},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("does emits a registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname3}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})

				It("modifies the passed in new entry with the same routes", func() {
					Expect(newEntry.Routes).To(ConsistOf(
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname3},
					))
				})
			})

			Context("when reemitting change to previous endpoints", func() {
				BeforeEach(func() {
					newEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname1},
							routing_table.Route{Hostname: hostname3},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("emits a registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname3}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})

				It("modifies the passed in new entry with the same routes", func() {
					Expect(newEntry.Routes).To(ConsistOf(
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname3},
					))
				})
			})
		})

		Context("when the domain is NOT fresh", func() {
			BeforeEach(func() {
				domains = noFreshDomains
			})

			Context("when reemitting the previous endpoints", func() {
				BeforeEach(func() {
					newEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname1},
							routing_table.Route{Hostname: hostname2},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("does emits an registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})

				It("modifies the passed in new entry with the same routes", func() {
					Expect(newEntry.Routes).To(ConsistOf(
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname2},
					))
				})

				Context("when the emitter only has the actual LRP state", func() {
					BeforeEach(func() {
						newEntry = &routing_table.RoutableEndpoints{
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}
					})

					It("emits the registration", func() {
						expected := routing_table.MessagesToEmit{
							RegistrationMessages: []routing_table.RegistryMessage{
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
							},
						}
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})
			})

			Context("when reemitting change to previous endpoints", func() {
				BeforeEach(func() {
					newEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname1},
							routing_table.Route{Hostname: hostname3},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("emits a merged registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname3}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})

				It("modifies the passed in entry with the merged routes", func() {
					Expect(newEntry.Routes).To(ConsistOf(
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname2},
						routing_table.Route{Hostname: hostname3},
					))
				})
			})
		})
	})

	Describe("RegistrationsFor", func() {
		BeforeEach(func() {
			existingEntry = nil

			newEntry = &routing_table.RoutableEndpoints{
				Routes: []routing_table.Route{
					routing_table.Route{Hostname: hostname1},
				},
				Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
			}
		})

		JustBeforeEach(func() {
			messages = builder.RegistrationsFor(existingEntry, newEntry)
		})

		Context("when no existing entry", func() {
			It("emits a registration", func() {
				expected := routing_table.MessagesToEmit{
					RegistrationMessages: []routing_table.RegistryMessage{
						routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
					},
				}
				Expect(messages).To(MatchMessagesToEmit(expected))
			})
		})

		Context("when new entry has no hostnames", func() {
			BeforeEach(func() {
				newEntry.Routes = []routing_table.Route{}
			})

			It("emits nothing", func() {
				Expect(messages).To(BeZero())
			})
		})

		Context("when we have an existing entry", func() {
			Context("when existing == new", func() {
				BeforeEach(func() {
					existingEntry = newEntry
				})

				It("emits nothing", func() {
					Expect(messages).To(BeZero())
				})
			})

			Context("when route service url changes", func() {
				BeforeEach(func() {
					existingEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{
								Hostname:        hostname1,
								RouteServiceUrl: "https://new-rs-url.com",
							},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("emits a registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when hostnames change", func() {
				BeforeEach(func() {
					existingEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname2},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("emits a registration", func() {
					expected := routing_table.MessagesToEmit{
						RegistrationMessages: []routing_table.RegistryMessage{
							routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
						},
					}
					Expect(messages).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when endpoints are changed", func() {
				Context("when endpoints are added", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry.Endpoints = routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1, endpoint2})
					})

					It("emits a registration", func() {
						expected := routing_table.MessagesToEmit{
							RegistrationMessages: []routing_table.RegistryMessage{
								routing_table.RegistryMessageFor(endpoint2, routing_table.Route{Hostname: hostname1}),
							},
						}
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})

				Context("when endpoints are removed", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1, endpoint2}),
						}

						newEntry.Endpoints = routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1})
					})

					It("emits nothing", func() {
						Expect(messages).To(BeZero())
					})
				})
			})
		})
	})

	Describe("UnregistrationsFor", func() {

		Context("when doing bulk sync loop", func() {
			BeforeEach(func() {
				existingEntry = &routing_table.RoutableEndpoints{
					Routes: []routing_table.Route{
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname2},
					},
					Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
				}

				newEntry = &routing_table.RoutableEndpoints{
					Routes: []routing_table.Route{
						routing_table.Route{Hostname: hostname1},
						routing_table.Route{Hostname: hostname2},
					},
					Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{}),
				}
			})

			Context("when domain is fresh", func() {
				BeforeEach(func() {
					domains := models.NewDomainSet([]string{"tests"})

					messages = builder.UnregistrationsFor(existingEntry, newEntry, domains)
				})

				Context("when an endpoint is removed", func() {
					It("emits an unregistration", func() {
						expected := routing_table.MessagesToEmit{
							UnregistrationMessages: []routing_table.RegistryMessage{
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
							},
						}
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})
			})

			Context("when the domain is NOT fresh", func() {
				BeforeEach(func() {
					domains := models.NewDomainSet([]string{"foo"})

					messages = builder.UnregistrationsFor(existingEntry, newEntry, domains)
				})

				Context("when an endpoint is removed", func() {
					It("does not emit an unregistration", func() {
						Expect(messages).To(BeZero())
					})
				})
			})

		})

		Context("when doing event processing", func() {

			JustBeforeEach(func() {
				messages = builder.UnregistrationsFor(existingEntry, newEntry, nil)
			})

			Context("when there are no hostnames in the existing", func() {
				BeforeEach(func() {
					existingEntry = &routing_table.RoutableEndpoints{
						Routes:    []routing_table.Route{},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}

					newEntry = &routing_table.RoutableEndpoints{
						Routes: []routing_table.Route{
							routing_table.Route{Hostname: hostname1},
						},
						Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
					}
				})

				It("emits nothing", func() {
					Expect(messages).To(BeZero())
				})
			})

			Context("when hostnames change", func() {
				Context("when a hostname removed", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = &routing_table.RoutableEndpoints{
							Routes:    []routing_table.Route{},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}
					})

					It("emits an unregistration", func() {
						expected := routing_table.MessagesToEmit{
							UnregistrationMessages: []routing_table.RegistryMessage{
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
							},
						}
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})

				Context("when a hostname has been added", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}
					})

					It("emits nothing", func() {
						Expect(messages).To(BeZero())
					})
				})

				Context("when a hostname has not changed", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = existingEntry
					})

					It("emits nothing", func() {
						Expect(messages).To(BeZero())
					})
				})
			})

			Context("when endpoints change", func() {
				Context("when an endpoint is removed", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{}),
						}
					})

					It("emits an unregistration", func() {
						expected := routing_table.MessagesToEmit{
							UnregistrationMessages: []routing_table.RegistryMessage{
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname1}),
								routing_table.RegistryMessageFor(endpoint1, routing_table.Route{Hostname: hostname2}),
							},
						}
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})

				Context("when an endpoint has been added", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
								routing_table.Route{Hostname: hostname2},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1, endpoint2}),
						}
					})

					It("emits nothing", func() {
						Expect(messages).To(BeZero())
					})
				})

				Context("when endpoints have not changed", func() {
					BeforeEach(func() {
						existingEntry = &routing_table.RoutableEndpoints{
							Routes: []routing_table.Route{
								routing_table.Route{Hostname: hostname1},
							},
							Endpoints: routing_table.EndpointsAsMap([]routing_table.Endpoint{endpoint1}),
						}

						newEntry = existingEntry
					})

					It("emits nothing", func() {
						Expect(messages).To(BeZero())
					})
				})
			})

		})
	})
})
