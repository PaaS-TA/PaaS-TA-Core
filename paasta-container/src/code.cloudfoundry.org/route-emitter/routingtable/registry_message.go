package routingtable

import "fmt"

type RegistryMessage struct {
	Host                 string            `json:"host"`
	Port                 uint32            `json:"port"`
	URIs                 []string          `json:"uris"`
	App                  string            `json:"app,omitempty"`
	RouteServiceUrl      string            `json:"route_service_url,omitempty"`
	PrivateInstanceId    string            `json:"private_instance_id,omitempty"`
	PrivateInstanceIndex string            `json:"private_instance_index,omitempty"`
	IsolationSegment     string            `json:"isolation_segment,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}

func RegistryMessageFor(endpoint Endpoint, route Route) RegistryMessage {
	var index string
	if endpoint.InstanceGUID != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	return RegistryMessage{
		URIs:             []string{route.Hostname},
		Host:             endpoint.Host,
		Port:             endpoint.Port,
		App:              route.LogGUID,
		IsolationSegment: route.IsolationSegment,
		Tags:             map[string]string{"component": "route-emitter"},

		PrivateInstanceId:    endpoint.InstanceGUID,
		PrivateInstanceIndex: index,
		RouteServiceUrl:      route.RouteServiceUrl,
	}
}

func InternalAddressRegistryMessageFor(endpoint Endpoint, route Route) RegistryMessage {
	var index string
	if endpoint.InstanceGUID != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	return RegistryMessage{
		URIs:             []string{route.Hostname},
		Host:             endpoint.ContainerIP,
		Port:             endpoint.ContainerPort,
		App:              route.LogGUID,
		IsolationSegment: route.IsolationSegment,
		Tags:             map[string]string{"component": "route-emitter"},

		PrivateInstanceId:    endpoint.InstanceGUID,
		PrivateInstanceIndex: index,
		RouteServiceUrl:      route.RouteServiceUrl,
	}
}

type RouterGreetingMessage struct {
	MinimumRegisterInterval int `json:"minimumRegisterIntervalInSeconds"`
	PruneThresholdInSeconds int `json:"pruneThresholdInSeconds"`
}
