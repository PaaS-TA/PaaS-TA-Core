package routing_table

import "code.cloudfoundry.org/bbs/models"

type MessageBuilder interface {
	RegistrationsFor(existingEntry, newEntry *RoutableEndpoints) MessagesToEmit
	UnfreshRegistrations(existingEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit
	MergedRegistrations(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit
	UnregistrationsFor(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit
}

type NoopMessageBuilder struct {
}

func (NoopMessageBuilder) RegistrationsFor(existingEntry, newEntry *RoutableEndpoints) MessagesToEmit {
	return MessagesToEmit{}
}

func (NoopMessageBuilder) UnfreshRegistrations(existingEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	return MessagesToEmit{}
}

func (NoopMessageBuilder) MergedRegistrations(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	return MessagesToEmit{}
}

func (NoopMessageBuilder) UnregistrationsFor(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	return MessagesToEmit{}
}

type MessagesToEmitBuilder struct {
}

func (MessagesToEmitBuilder) UnfreshRegistrations(existingEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	messagesToEmit := MessagesToEmit{}
	for _, endpoint := range existingEntry.Endpoints {
		if domains != nil && !domains.Contains(endpoint.Domain) {
			createAndAddMessages(endpoint, existingEntry.Routes, &messagesToEmit.RegistrationMessages)
		}
	}

	return messagesToEmit
}

func (MessagesToEmitBuilder) MergedRegistrations(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	messagesToEmit := MessagesToEmit{}

	for _, endpoint := range newEntry.Endpoints {
		routeList := newEntry.Routes
		if domains != nil && !domains.Contains(endpoint.Domain) {
			// Not Fresh
			for _, route := range existingEntry.Routes {
				var addRoute bool = true
				for _, newRoute := range routeList {
					if newRoute.Hostname == route.Hostname {
						addRoute = false
					}
				}
				if addRoute {
					routeList = append(routeList, route)
				}
			}
		}
		newEntry.Routes = routeList

		if len(newEntry.Routes) == 0 {
			//no hostnames, so nothing could possibly be registered
			continue
		}

		createAndAddMessages(endpoint, routeList, &messagesToEmit.RegistrationMessages)
	}
	return messagesToEmit
}

func (MessagesToEmitBuilder) RegistrationsFor(existingEntry, newEntry *RoutableEndpoints) MessagesToEmit {
	messagesToEmit := MessagesToEmit{}
	if len(newEntry.Routes) == 0 {
		//no hostnames, so nothing could possibly be registered
		return messagesToEmit
	}

	// only new entry OR something changed between existing and new entry
	if existingEntry == nil || hostnamesHaveChanged(existingEntry, newEntry) || routeServiceUrlHasChanged(existingEntry, newEntry) {
		for _, endpoint := range newEntry.Endpoints {
			createAndAddMessages(endpoint, newEntry.Routes, &messagesToEmit.RegistrationMessages)
		}
		return messagesToEmit
	}

	//otherwise only register *new* endpoints
	for _, endpoint := range newEntry.Endpoints {
		if !existingEntry.hasEndpoint(endpoint) {
			createAndAddMessages(endpoint, newEntry.Routes, &messagesToEmit.RegistrationMessages)
		}
	}

	return messagesToEmit
}

func (MessagesToEmitBuilder) UnregistrationsFor(existingEntry, newEntry *RoutableEndpoints, domains models.DomainSet) MessagesToEmit {
	messagesToEmit := MessagesToEmit{}

	if len(existingEntry.Routes) == 0 {
		// the existing entry has no hostnames and so there is nothing to unregister
		return messagesToEmit
	}

	endpointsThatAreStillPresent := []Endpoint{}
	for _, endpoint := range existingEntry.Endpoints {
		if newEntry.hasEndpoint(endpoint) {
			endpointsThatAreStillPresent = append(endpointsThatAreStillPresent, endpoint)
		} else {
			// only unregister if domain is fresh or preforming event processing
			if domains == nil || domains.Contains(endpoint.Domain) {
				//if the endpoint has disappeared unregister all its previous hostnames
				createAndAddMessages(endpoint, existingEntry.Routes, &messagesToEmit.UnregistrationMessages)
			}
		}
	}

	routesThatDisappeared := []Route{}
	for _, route := range existingEntry.Routes {
		if !newEntry.hasHostname(route.Hostname) {
			routesThatDisappeared = append(routesThatDisappeared, route)
		}
	}

	if len(routesThatDisappeared) > 0 {
		for _, endpoint := range endpointsThatAreStillPresent {
			// only unregister if domain is fresh or preforming event processing
			if domains == nil || domains.Contains(endpoint.Domain) {
				//if a endpoint is still present, and hostnames have disappeared, unregister those hostnames
				createAndAddMessages(endpoint, routesThatDisappeared, &messagesToEmit.UnregistrationMessages)
			}
		}
	}

	return messagesToEmit
}

func hostnamesHaveChanged(existingEntry, newEntry *RoutableEndpoints) bool {
	if len(newEntry.Routes) != len(existingEntry.Routes) {
		return true
	} else {
		for _, route := range newEntry.Routes {
			if !existingEntry.hasHostname(route.Hostname) {
				return true
			}
		}
	}

	return false
}

func routeServiceUrlHasChanged(existingEntry, newEntry *RoutableEndpoints) bool {
	if len(newEntry.Routes) != len(existingEntry.Routes) {
		return true
	} else {
		for _, route := range newEntry.Routes {
			if !existingEntry.hasRouteServiceUrl(route.RouteServiceUrl) {
				return true
			}
		}
	}
	return false
}

func createAndAddMessages(endpoint Endpoint, routes []Route, messages *[]RegistryMessage) {
	for _, route := range routes {
		message := RegistryMessageFor(endpoint, route)
		*messages = append(*messages, message)
	}
}
