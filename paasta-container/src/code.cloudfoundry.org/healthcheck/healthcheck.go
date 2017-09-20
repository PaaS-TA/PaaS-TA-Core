package healthcheck

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

type HealthCheckError struct {
	Code    int
	Message string
}

func (e HealthCheckError) Error() string {
	return e.Message
}

type HealthCheck struct {
	network string
	uri     string
	port    string
	timeout time.Duration
}

func NewHealthCheck(network, uri, port string, timeout time.Duration) HealthCheck {
	return HealthCheck{network, uri, port, timeout}
}

func (h *HealthCheck) CheckInterfaces(interfaces []net.Interface) error {
	healthcheck := h.HTTPHealthCheck
	if len(h.uri) == 0 {
		healthcheck = h.PortHealthCheck
	}

	for _, intf := range interfaces {
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				err := healthcheck(ipnet.IP.String())
				return err
			}
		}
	}

	return HealthCheckError{Code: 3, Message: "failure to find suitable interface"}
}

func (h *HealthCheck) PortHealthCheck(ip string) error {
	addr := ip + ":" + h.port
	conn, err := net.DialTimeout(h.network, addr, h.timeout)
	if err == nil {
		conn.Close()
		return nil
	}

	if err, ok := err.(net.Error); ok && err.Timeout() {
		return HealthCheckError{Code: 64, Message: fmt.Sprintf("timeout when making TCP connection: %s", err)}
	}

	return HealthCheckError{Code: 4, Message: fmt.Sprintf("failure to make TCP connection: %s", err)}
}

func (h *HealthCheck) HTTPHealthCheck(ip string) error {
	addr := fmt.Sprintf("http://%s:%s%s", ip, h.port, h.uri)
	client := http.Client{
		Timeout: h.timeout,
	}
	resp, err := client.Get(addr)
	if err == nil {
		if resp.StatusCode == http.StatusOK {
			return nil
		}

		return HealthCheckError{Code: 6, Message: fmt.Sprintf("failure to get valid HTTP status code: %d", resp.StatusCode)}
	}

	if err, ok := err.(net.Error); ok && err.Timeout() {
		return HealthCheckError{Code: 65, Message: fmt.Sprintf("timeout when making HTTP request: %s", err)}
	}

	return HealthCheckError{Code: 5, Message: fmt.Sprintf("failure to make HTTP request: %s", err)}
}
