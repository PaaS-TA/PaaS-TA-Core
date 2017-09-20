package endpoint

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type EndpointKey struct {
	InstanceGUID string
	Evacuating   bool
}

func NewEndpointKey(instanceGUID string, evacuating bool) EndpointKey {
	return EndpointKey{
		InstanceGUID: instanceGUID,
		Evacuating:   evacuating,
	}
}

type Endpoint struct {
	InstanceGUID    string
	Host            string
	Port            uint32
	ContainerPort   uint32
	Evacuating      bool
	ModificationTag *models.ModificationTag
}

func (e Endpoint) Key() EndpointKey {
	return EndpointKey{InstanceGUID: e.InstanceGUID, Evacuating: e.Evacuating}
}

func NewEndpoint(
	instanceGUID string, evacuating bool,
	host string, port, containerPort uint32,
	modificationTag *models.ModificationTag) Endpoint {
	return Endpoint{
		InstanceGUID:    instanceGUID,
		Evacuating:      evacuating,
		Host:            host,
		Port:            port,
		ContainerPort:   containerPort,
		ModificationTag: modificationTag,
	}
}

type ExternalEndpointInfo struct {
	RouterGroupGUID string
	Port            uint32
}

type ExternalEndpointInfos []ExternalEndpointInfo

func NewExternalEndpointInfo(routerGroupGUID string, port uint32) ExternalEndpointInfo {
	return ExternalEndpointInfo{
		RouterGroupGUID: routerGroupGUID,
		Port:            port,
	}
}

type RoutableEndpoints struct {
	ExternalEndpoints ExternalEndpointInfos
	Endpoints         map[EndpointKey]Endpoint
	LogGUID           string
	ModificationTag   *models.ModificationTag
}

func (entry RoutableEndpoints) Copy() RoutableEndpoints {
	clone := RoutableEndpoints{
		ExternalEndpoints: entry.ExternalEndpoints,
		Endpoints:         map[EndpointKey]Endpoint{},
		LogGUID:           entry.LogGUID,
		ModificationTag:   entry.ModificationTag,
	}

	for k, v := range entry.Endpoints {
		clone.Endpoints[k] = v
	}

	return clone
}

func NewEndpointsFromActual(actualGrp *models.ActualLRPGroup) map[uint32]Endpoint {
	endpoints := map[uint32]Endpoint{}
	actual, evacuating := actualGrp.Resolve()

	for _, portMapping := range actual.Ports {
		endpoint := NewEndpoint(
			actual.InstanceGuid, evacuating,
			actual.Address,
			portMapping.HostPort,
			portMapping.ContainerPort,
			&actual.ModificationTag,
		)
		endpoints[portMapping.ContainerPort] = endpoint
	}

	return endpoints
}

func NewRoutingKeysFromActual(actualGrp *models.ActualLRPGroup) RoutingKeys {
	keys := RoutingKeys{}
	actual, _ := actualGrp.Resolve()
	for _, portMapping := range actual.Ports {
		keys = append(keys, NewRoutingKey(actual.ProcessGuid, portMapping.ContainerPort))
	}

	return keys
}

func NewRoutingKeysFromDesired(desired *models.DesiredLRP) RoutingKeys {
	keys := RoutingKeys{}
	for _, containerPort := range desired.Ports {
		keys = append(keys, NewRoutingKey(desired.ProcessGuid, containerPort))
	}

	return keys
}

func (e ExternalEndpointInfos) HasNoExternalPorts(logger lager.Logger) bool {
	if e == nil || len(e) == 0 {
		logger.Debug("no-external-port")
		return true
	}
	// This originally checked if Port was 0, I think to see if it was a zero value, check and make sure
	return false
}

func (e RoutableEndpoints) HaveEndpointsChanged(newEntry RoutableEndpoints) bool {
	if len(e.Endpoints) != len(newEntry.Endpoints) {
		// length not same...so something changed
		return true
	}
	//Check if new endpoints are added or existing endpoints are modified
	for key, newEndpoint := range newEntry.Endpoints {
		if existingEndpoint, ok := e.Endpoints[key]; !ok {
			// new endpoint
			return true
		} else {
			if existingEndpoint.ModificationTag.SucceededBy(newEndpoint.ModificationTag) {
				// existing endpoint modified
				return true
			}
		}
	}
	return false
}

func (e RoutableEndpoints) HaveExternalEndpointsChanged(newEntry RoutableEndpoints) bool {
	if len(e.ExternalEndpoints) != len(newEntry.ExternalEndpoints) {
		// length not same...so something changed
		return true
	}
	//Check if new endpoints are added
	for _, existing := range e.ExternalEndpoints {
		found := false
		for _, proposed := range newEntry.ExternalEndpoints {
			if proposed.Port == existing.Port {
				found = true
				break
			}
		}

		// Could not find existing endpoint, something changed
		if !found {
			return true
		}
	}
	return false
}

func NewRoutableEndpoints(
	externalEndPoint ExternalEndpointInfos,
	endpoints map[EndpointKey]Endpoint,
	logGUID string,
	modificationTag *models.ModificationTag) RoutableEndpoints {
	return RoutableEndpoints{
		ExternalEndpoints: externalEndPoint,
		Endpoints:         endpoints,
		LogGUID:           logGUID,
		ModificationTag:   modificationTag,
	}
}

type RoutingKeys []RoutingKey

type RoutingKey struct {
	ProcessGUID   string
	ContainerPort uint32
}

func NewRoutingKey(processGUID string, containerPort uint32) RoutingKey {
	return RoutingKey{
		ProcessGUID:   processGUID,
		ContainerPort: containerPort,
	}
}

// this function returns the entry with the external externalEndpoints substracted from its internal collection
// Ex; Given, entry { externalEndpoints={p1,p2,p4} } and externalEndpoints = {p2,p3} ==> entryA { externalEndpoints={p1,p4} }
func (entry RoutableEndpoints) RemoveExternalEndpoints(externalEndpoints ExternalEndpointInfos) RoutableEndpoints {
	subtractedExternalEndpoints := entry.ExternalEndpoints.Remove(externalEndpoints)
	resultEntry := entry.Copy()
	resultEntry.ExternalEndpoints = subtractedExternalEndpoints
	return resultEntry
}

// this function return a-b set. Ex: a = {p1,p2, p4} b={p2,p3} ===> a-b = {p1, p4}
func (setA ExternalEndpointInfos) Remove(setB ExternalEndpointInfos) ExternalEndpointInfos {
	diffSet := ExternalEndpointInfos{}
	for _, externalEndpoint := range setA {
		if !setB.ContainsExternalPort(externalEndpoint.Port) {
			diffSet = append(diffSet, ExternalEndpointInfo{externalEndpoint.RouterGroupGUID, externalEndpoint.Port})
		}
	}
	return diffSet
}

func (e ExternalEndpointInfos) ContainsExternalPort(port uint32) bool {
	for _, existing := range e {
		if existing.Port == port {
			return true
		}
	}
	return false
}

func (lhs RoutingKeys) Remove(rhs RoutingKeys) RoutingKeys {
	result := RoutingKeys{}
	for _, lhsKey := range lhs {
		if !rhs.containsRoutingKey(lhsKey) {
			result = append(result, lhsKey)
		}
	}
	return result
}

func (lhs RoutingKeys) containsRoutingKey(routingKey RoutingKey) bool {
	for _, lhsKey := range lhs {
		if lhsKey == routingKey {
			return true
		}
	}
	return false
}
