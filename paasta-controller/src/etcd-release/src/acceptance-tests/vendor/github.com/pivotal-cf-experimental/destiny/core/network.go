package core

import (
	"errors"
	"fmt"
	"strings"
)

type Network struct {
	Name    string          `yaml:"name"`
	Subnets []NetworkSubnet `yaml:"subnets"`
	Type    string          `yaml:"type"`
}

type NetworkSubnet struct {
	CloudProperties NetworkSubnetCloudProperties `yaml:"cloud_properties"`
	Gateway         string                       `yaml:"gateway"`
	Range           string                       `yaml:"range"`
	Reserved        []string                     `yaml:"reserved"`
	Static          []string                     `yaml:"static"`
}

type NetworkSubnetCloudProperties struct {
	Name           string   `yaml:"name"`
	Subnet         string   `yaml:"subnet,omitempty"`
	SecurityGroups []string `yaml:"security_groups,omitempty"`
}

func (n Network) StaticIPs(count int) []string {
	var ips []string
	for _, subnet := range n.Subnets {
		ips = append(ips, subnet.Static...)
	}

	if len(ips) >= count {
		return ips[:count]
	}

	return []string{}
}

func (n Network) StaticIPsFromRange(count int) ([]string, error) {
	if count < 0 {
		return []string{}, errors.New("count must be greater than or equal to zero")
	}

	var ips []string
	for _, subnet := range n.Subnets {
		subnetIPs, err := n.rangeToList(subnet.Static[0])
		if err != nil {
			return nil, err
		}

		ips = append(ips, subnetIPs...)
	}

	if len(ips) >= count {
		return ips[:count], nil
	}

	return []string{}, fmt.Errorf("can't allocate %d ips from %d available ips", count, len(ips))
}

func (n Network) rangeToList(ipRange string) ([]string, error) {
	ipRange = strings.Replace(ipRange, " ", "", -1)

	ips := strings.Split(ipRange, "-")

	if len(ips) != 2 {
		return nil, errors.New("static ip's must be a range in the form of x.x.x.x-x.x.x.x")
	}

	firstIP, err := ParseIP(ips[0])
	if err != nil {
		return nil, err
	}

	secondIP, err := ParseIP(ips[1])
	if err != nil {
		return nil, err
	}

	return firstIP.To(secondIP), nil
}
