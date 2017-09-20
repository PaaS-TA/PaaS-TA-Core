package testrunner

import (
	"os/exec"
	"strconv"

	"github.com/tedsuo/ifrit/ginkgomon"
)

type Args struct {
	Port       uint16
	ConfigPath string
	DevMode    bool
	IP         string
}

func (args Args) ArgSlice() []string {
	return []string{
		"-port", strconv.Itoa(int(args.Port)),
		"-ip", args.IP,
		"-config", args.ConfigPath,
		"-logLevel=debug",
		"-devMode=" + strconv.FormatBool(args.DevMode),
	}
}

func New(binPath string, args Args) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:       "routing-api",
		Command:    exec.Command(binPath, args.ArgSlice()...),
		StartCheck: "started",
	})
}
