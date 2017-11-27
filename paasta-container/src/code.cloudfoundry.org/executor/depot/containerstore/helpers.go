package containerstore

import (
	"errors"
	"net"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/garden"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager"
)

var ErrIPRangeConversionFailed = errors.New("failed to convert destination to ip range")

func logStreamerFromLogConfig(conf executor.LogConfig, metronClient loggregator_v2.IngressClient) log_streamer.LogStreamer {
	return log_streamer.New(
		conf.Guid,
		conf.SourceName,
		conf.Index,
		metronClient,
	)
}

func newBindMount(src, dst string) garden.BindMount {
	return garden.BindMount{
		SrcPath: src,
		DstPath: dst,
		Mode:    garden.BindMountModeRO,
		Origin:  garden.BindMountOriginHost,
	}
}

func convertDiskScope(scope executor.DiskLimitScope) garden.DiskLimitScope {
	if scope == executor.TotalDiskLimit {
		return garden.DiskLimitScopeTotal
	}
	return garden.DiskLimitScopeExclusive
}

func convertEnvVars(execEnv []executor.EnvironmentVariable) []string {
	env := make([]string, len(execEnv))
	for i := range execEnv {
		envVar := &execEnv[i]
		env[i] = envVar.Name + "=" + envVar.Value
	}
	return env
}

func convertEgressToNetOut(logger lager.Logger, egressRules []*models.SecurityGroupRule) ([]garden.NetOutRule, error) {
	netOutRules := make([]garden.NetOutRule, len(egressRules))
	for i, rule := range egressRules {
		if err := rule.Validate(); err != nil {
			logger.Error("invalid-egress-rule", err)
			return nil, err
		}

		netOutRule, err := securityGroupRuleToNetOutRule(rule)
		if err != nil {
			logger.Error("failed-to-convert-to-net-out-rule", err)
			return nil, err
		}

		netOutRules[i] = netOutRule
	}
	return netOutRules, nil
}

func securityGroupRuleToNetOutRule(securityRule *models.SecurityGroupRule) (garden.NetOutRule, error) {
	var protocol garden.Protocol
	var portRanges []garden.PortRange
	var networks []garden.IPRange
	var icmp *garden.ICMPControl

	switch securityRule.Protocol {
	case models.TCPProtocol:
		protocol = garden.ProtocolTCP
	case models.UDPProtocol:
		protocol = garden.ProtocolUDP
	case models.ICMPProtocol:
		protocol = garden.ProtocolICMP
		icmp = &garden.ICMPControl{
			Type: garden.ICMPType(securityRule.IcmpInfo.Type),
			Code: garden.ICMPControlCode(uint8(securityRule.IcmpInfo.Code)),
		}
	case models.AllProtocol:
		protocol = garden.ProtocolAll
	}

	if securityRule.PortRange != nil {
		portRanges = append(portRanges, garden.PortRange{Start: uint16(securityRule.PortRange.Start), End: uint16(securityRule.PortRange.End)})
	} else if securityRule.Ports != nil {
		for _, port := range securityRule.Ports {
			portRanges = append(portRanges, garden.PortRangeFromPort(uint16(port)))
		}
	}

	for _, dest := range securityRule.Destinations {
		ipRange, err := toIPRange(dest)
		if err != nil {
			return garden.NetOutRule{}, err
		}
		networks = append(networks, ipRange)
	}

	netOutRule := garden.NetOutRule{
		Protocol: protocol,
		Networks: networks,
		Ports:    portRanges,
		ICMPs:    icmp,
		Log:      securityRule.Log,
	}

	return netOutRule, nil
}

func toIPRange(dest string) (garden.IPRange, error) {
	idx := strings.IndexAny(dest, "-/")

	// Not a range or a CIDR
	if idx == -1 {
		ip := net.ParseIP(dest)
		if ip == nil {
			return garden.IPRange{}, ErrIPRangeConversionFailed
		}

		return garden.IPRangeFromIP(ip), nil
	}

	// We have a CIDR
	if dest[idx] == '/' {
		_, ipNet, err := net.ParseCIDR(dest)
		if err != nil {
			return garden.IPRange{}, ErrIPRangeConversionFailed
		}

		return garden.IPRangeFromIPNet(ipNet), nil
	}

	// We have an IP range
	firstIP := net.ParseIP(dest[:idx])
	secondIP := net.ParseIP(dest[idx+1:])
	if firstIP == nil || secondIP == nil {
		return garden.IPRange{}, ErrIPRangeConversionFailed
	}

	return garden.IPRange{Start: firstIP, End: secondIP}, nil
}
