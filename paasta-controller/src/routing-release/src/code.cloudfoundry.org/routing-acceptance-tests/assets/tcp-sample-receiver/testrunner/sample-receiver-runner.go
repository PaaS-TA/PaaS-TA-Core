package testrunner

import (
	"os/exec"
	"time"

	"github.com/tedsuo/ifrit/ginkgomon"
)

type Args struct {
	Address  string
	ServerId string
}

func (args Args) ArgSlice() []string {
	return []string{
		"-address=" + args.Address,
		"-serverId=" + args.ServerId,
	}
}

func New(binPath string, args Args) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "sample-receiver",
		AnsiColorCode:     "1;96m",
		StartCheck:        "Listening on",
		StartCheckTimeout: 10 * time.Second,
		Command:           exec.Command(binPath, args.ArgSlice()...),
	})
}
