package main

import (
	"log"
	"os"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcdfab/application"
	"github.com/cloudfoundry-incubator/etcd-release/src/etcdfab/command"

	"code.cloudfoundry.org/lager"
)

// var generateCommand func(name string, arg ...string) *Cmd
// var generateCommand func(name string, arg ...string) *exec.Cmd
// var generateCommand = exec.Command

func main() {
	etcdPidPath := os.Args[1]
	configFilePath := os.Args[2]
	etcdArgs := os.Args[3:]

	commandWrapper := command.NewWrapper()

	logger := lager.NewLogger("etcdfab")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	app := application.New(application.NewArgs{
		Command:        commandWrapper,
		CommandPidPath: etcdPidPath,
		ConfigFilePath: configFilePath,
		EtcdArgs:       etcdArgs,
		OutWriter:      os.Stdout,
		ErrWriter:      os.Stderr,
		Logger:         logger,
	})

	err := app.Start()
	if err != nil {
		stderr := log.New(os.Stderr, "", 0)
		stderr.Printf("error during start: %s", err)
		os.Exit(1)
	}
}
