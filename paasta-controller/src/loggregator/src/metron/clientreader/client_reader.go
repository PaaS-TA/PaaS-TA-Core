package clientreader

import (
	"doppler/dopplerservice"
	"fmt"
)

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	SetAddresses(addresses []string) int
}

func Read(clientPool map[string]ClientPool, protocols []string, event dopplerservice.Event) string {
	protocol, servers := chooseProtocol(protocols, event)
	if protocol == "" {
		panic(fmt.Sprintf("No dopplers listening on %v", protocols))
	}
	clients := clientPool[protocol].SetAddresses(servers)
	if clients == 0 {
		panic(fmt.Sprintf("Unable to connect to dopplers running on %s", protocol))
	}
	return protocol
}

func chooseProtocol(protocols []string, event dopplerservice.Event) (string, []string) {
	for _, protocol := range protocols {
		var dopplers []string
		fmt.Println("### client_reader.go : chooseProtocol - protocol : ", protocol)
		switch protocol {
		case "udp":
			fmt.Println("### client_reader.go : chooseProtocol - udp")
			dopplers = event.UDPDopplers
		case "tcp":
			fmt.Println("### client_reader.go : chooseProtocol - tcp")
			dopplers = event.TCPDopplers
		case "tls":
			fmt.Println("### client_reader.go : chooseProtocol - tls")
			dopplers = event.TLSDopplers
		}
		if len(dopplers) > 0 {
			return protocol, dopplers
		}
	}
	return "", nil
}
