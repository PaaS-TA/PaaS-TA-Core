package haproxy

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/cf-tcp-router"
	"code.cloudfoundry.org/cf-tcp-router/models"
)

func BackendServerInfoToHaProxyConfig(bs models.BackendServerInfo) (string, error) {
	if bs.Address == "" {
		return "", cf_tcp_router.ErrInvalidField{Field: "backend_server.address"}
	}
	if bs.Port == 0 {
		return "", cf_tcp_router.ErrInvalidField{Field: "backend_server.port"}
	}
	name := fmt.Sprintf("server_%s_%d", bs.Address, bs.Port)
	return fmt.Sprintf("server %s %s:%d\n", name, bs.Address, bs.Port), nil
}

func RoutingTableEntryToHaProxyConfig(routingKey models.RoutingKey, routingTableEntry models.RoutingTableEntry) (string, error) {
	if routingKey.Port == 0 {
		return "", cf_tcp_router.ErrInvalidField{Field: "listen_configuration.port"}
	}
	if len(routingTableEntry.Backends) == 0 {
		return "", cf_tcp_router.ErrInvalidField{Field: "listen_configuration.backends"}
	}
	name := fmt.Sprintf("listen_cfg_%d", routingKey.Port)
	var buff bytes.Buffer

	buff.WriteString(fmt.Sprintf("listen %s\n  mode tcp\n  bind :%d\n", name, routingKey.Port))
	for bskey, bsdetails := range routingTableEntry.Backends {
		bs := models.NewBackendServerInfo(bskey, bsdetails)
		str, err := BackendServerInfoToHaProxyConfig(bs)
		if err != nil {
			return "", err
		}
		buff.WriteString(fmt.Sprintf("  %s", str))
	}
	return buff.String(), nil
}
