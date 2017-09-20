package routing_table

import "fmt"

type RegistryMessage struct {
	Host                 string            `json:"host"`
	Port                 uint32            `json:"port"`
	URIs                 []string          `json:"uris"`
	App                  string            `json:"app,omitempty"`
	RouteServiceUrl      string            `json:"route_service_url,omitempty"`
	PrivateInstanceId    string            `json:"private_instance_id,omitempty"`
	PrivateInstanceIndex string            `json:"private_instance_index,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}

func RegistryMessageFor(endpoint Endpoint, route Route) RegistryMessage {
	var index string
	if endpoint.InstanceGuid != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	return RegistryMessage{
		URIs: []string{route.Hostname},
		Host: endpoint.Host,
		Port: endpoint.Port,
		App:  route.LogGuid,
		Tags: map[string]string{"component": "route-emitter"},

		PrivateInstanceId:    endpoint.InstanceGuid,
		PrivateInstanceIndex: index,
		RouteServiceUrl:      route.RouteServiceUrl,
	}
}

type RouterGreetingMessage struct {
	MinimumRegisterInterval int `json:"minimumRegisterIntervalInSeconds"`
	PruneThresholdInSeconds int `json:"pruneThresholdInSeconds"`
}
