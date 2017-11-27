package routingtable_test

import (
	"encoding/json"

	"code.cloudfoundry.org/route-emitter/routingtable"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistryMessage", func() {
	var expectedMessage routingtable.RegistryMessage

	BeforeEach(func() {
		expectedMessage = routingtable.RegistryMessage{
			Host:                 "1.1.1.1",
			Port:                 61001,
			URIs:                 []string{"host-1.example.com"},
			App:                  "app-guid",
			PrivateInstanceId:    "instance-guid",
			PrivateInstanceIndex: "0",
			RouteServiceUrl:      "https://hello.com",
			Tags:                 map[string]string{"component": "route-emitter"},
		}
	})

	Describe("serialization", func() {
		var expectedJSON string

		BeforeEach(func() {
			expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"private_instance_index": "0",
				"route_service_url": "https://hello.com",
				"tags": {"component":"route-emitter"}
			}`
		})

		It("marshals correctly", func() {
			payload, err := json.Marshal(expectedMessage)
			Expect(err).NotTo(HaveOccurred())

			Expect(payload).To(MatchJSON(expectedJSON))
		})

		It("unmarshals correctly", func() {
			message := routingtable.RegistryMessage{}

			err := json.Unmarshal([]byte(expectedJSON), &message)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Describe("RegistryMessageFor", func() {
		It("creates a valid message from an endpoint and routes", func() {
			endpoint := routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         0,
				Host:          "1.1.1.1",
				Port:          61001,
				ContainerPort: 11,
			}
			route := routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}

			message := routingtable.RegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message when instance index is greater than 0", func() {
			endpoint := routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         2,
				Host:          "1.1.1.1",
				Port:          61001,
				ContainerPort: 11,
			}
			route := routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}

			expectedMessage.PrivateInstanceIndex = "2"
			message := routingtable.RegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Describe("InternalAddressRegistryMessageFor", func() {
		BeforeEach(func() {
			expectedMessage = routingtable.RegistryMessage{
				Host:                 "1.2.3.4",
				Port:                 11,
				URIs:                 []string{"host-1.example.com"},
				App:                  "app-guid",
				PrivateInstanceId:    "instance-guid",
				PrivateInstanceIndex: "0",
				RouteServiceUrl:      "https://hello.com",
				Tags:                 map[string]string{"component": "route-emitter"},
			}
		})

		It("creates a valid message from an endpoint and routes", func() {
			endpoint := routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         0,
				Host:          "1.1.1.1",
				ContainerIP:   "1.2.3.4",
				Port:          61001,
				ContainerPort: 11,
			}
			route := routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}

			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message when instance index is greater than 0", func() {
			endpoint := routingtable.Endpoint{
				InstanceGUID:  "instance-guid",
				Index:         2,
				Host:          "1.1.1.1",
				ContainerIP:   "1.2.3.4",
				Port:          61001,
				ContainerPort: 11,
			}
			route := routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
			}

			expectedMessage.PrivateInstanceIndex = "2"
			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route)
			Expect(message).To(Equal(expectedMessage))
		})
	})
})
