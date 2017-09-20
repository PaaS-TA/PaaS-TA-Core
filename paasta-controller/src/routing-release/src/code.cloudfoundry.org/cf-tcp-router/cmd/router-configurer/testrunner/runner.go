package testrunner

import (
	"os/exec"
	"time"

	"github.com/tedsuo/ifrit/ginkgomon"
)

// Args used by runner
type Args struct {
	BaseLoadBalancerConfigFilePath string
	LoadBalancerConfigFilePath     string
	ConfigFilePath                 string
}

func (args Args) ArgSlice() []string {
	return []string{
		"-tcpLoadBalancerConfig=" + args.LoadBalancerConfigFilePath,
		"-tcpLoadBalancerBaseConfig=" + args.BaseLoadBalancerConfigFilePath,
		"-config=" + args.ConfigFilePath,
		"-tokenFetchRetryInterval", "1s",
		"-staleRouteCheckInterval", "5s",
		"-logLevel=debug",
	}
}

func New(binPath string, args Args) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "router-configurer",
		AnsiColorCode:     "1;97m",
		StartCheck:        "router-configurer.started",
		StartCheckTimeout: 10 * time.Second,
		Command:           exec.Command(binPath, args.ArgSlice()...),
	})
}
