package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: check-a-record <host>")
		os.Exit(1)
	}

	host := os.Args[1]
	ips, err := net.LookupIP(host)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No A records found (%s)\n", err.Error())
		os.Exit(1)
	}

	filteredIPV4s := ipV4s(ips)
	if len(filteredIPV4s) == 0 {
		fmt.Fprintf(os.Stderr, "No A records found\n")
		os.Exit(1)
	}

	printIPs(filteredIPV4s)
}

func ipV4s(ips []net.IP) []net.IP {
	ipV4s := []net.IP{}
	for _, ip := range ips {
		if ipV4 := ip.To4(); ipV4 != nil {
			ipV4s = append(ipV4s, ipV4)
		}
	}

	return ipV4s
}

func printIPs(ips []net.IP) {
	for _, ip := range ips {
		fmt.Println(ip)
	}
}
