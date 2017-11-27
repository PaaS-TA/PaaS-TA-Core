package application

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/etcdfab/config"

	"code.cloudfoundry.org/lager"
)

type Application struct {
	command        command
	commandPidPath string
	configFilePath string
	etcdArgs       []string
	outWriter      io.Writer
	errWriter      io.Writer
	logger         logger
}

type command interface {
	Start(string, []string, io.Writer, io.Writer) (int, error)
}

type logger interface {
	Info(string, ...lager.Data)
	Error(string, error, ...lager.Data)
}

type NewArgs struct {
	Command        command
	CommandPidPath string
	ConfigFilePath string
	EtcdArgs       []string
	OutWriter      io.Writer
	ErrWriter      io.Writer
	Logger         logger
}

func New(args NewArgs) Application {
	return Application{
		command:        args.Command,
		commandPidPath: args.CommandPidPath,
		configFilePath: args.ConfigFilePath,
		etcdArgs:       args.EtcdArgs,
		outWriter:      args.OutWriter,
		errWriter:      args.ErrWriter,
		logger:         args.Logger,
	}
}

func (a Application) Start() error {
	cfg, err := config.ConfigFromJSON(a.configFilePath)
	if err != nil {
		a.logger.Error("application.read-config-file.failed", err)
		return err
	}

	etcdArgs := a.buildEtcdArgs(cfg)

	pid, err := a.command.Start(cfg.Etcd.EtcdPath, etcdArgs, a.outWriter, a.errWriter)
	if err != nil {
		a.logger.Error("application.start.failed", err)
		return err
	}

	err = ioutil.WriteFile(a.commandPidPath, []byte(fmt.Sprintf("%d", pid)), 0644)
	if err != nil {
		a.logger.Error("application.write-pid-file.failed", err)
		return err
	}

	return nil
}

func (a Application) buildEtcdArgs(cfg config.Config) []string {
	nodeName := fmt.Sprintf("%s-%d", strings.Replace(cfg.Node.Name, "_", "-", -1), cfg.Node.Index)
	a.logger.Info("application.build-etcd-flags", lager.Data{"node-name": nodeName})

	peerProtocol := "http"
	if cfg.Etcd.PeerRequireSSL {
		peerProtocol = "https"
	}

	clientProtocol := "http"
	if cfg.Etcd.RequireSSL {
		clientProtocol = "https"
	}

	peerUrl := fmt.Sprintf("http://%s:7001", cfg.Node.ExternalIP)
	if cfg.Etcd.PeerRequireSSL || cfg.Etcd.RequireSSL {
		peerUrl = fmt.Sprintf("https://%s.%s:7001", nodeName, cfg.Etcd.AdvertiseURLsDNSSuffix)
	}

	clientUrl := fmt.Sprintf("http://%s:4001", cfg.Node.ExternalIP)
	if cfg.Etcd.PeerRequireSSL || cfg.Etcd.RequireSSL {
		clientUrl = fmt.Sprintf("https://%s.%s:4001", nodeName, cfg.Etcd.AdvertiseURLsDNSSuffix)
	}

	etcdArgs := append(a.etcdArgs, "--name")
	etcdArgs = append(etcdArgs, nodeName)

	etcdArgs = append(etcdArgs, "--data-dir")
	etcdArgs = append(etcdArgs, "/var/vcap/store/etcd")

	etcdArgs = append(etcdArgs, "--heartbeat-interval")
	etcdArgs = append(etcdArgs, fmt.Sprintf("%d", cfg.Etcd.HeartbeatInterval))

	etcdArgs = append(etcdArgs, "--election-timeout")
	etcdArgs = append(etcdArgs, fmt.Sprintf("%d", cfg.Etcd.ElectionTimeout))

	etcdArgs = append(etcdArgs, "--listen-peer-urls")
	etcdArgs = append(etcdArgs, fmt.Sprintf("%s://%s:7001", peerProtocol, cfg.Etcd.PeerIP))

	etcdArgs = append(etcdArgs, "--listen-client-urls")
	etcdArgs = append(etcdArgs, fmt.Sprintf("%s://%s:4001", clientProtocol, cfg.Etcd.ClientIP))

	etcdArgs = append(etcdArgs, "--initial-advertise-peer-urls")
	etcdArgs = append(etcdArgs, peerUrl)

	etcdArgs = append(etcdArgs, "--advertise-client-urls")
	etcdArgs = append(etcdArgs, clientUrl)

	return etcdArgs
}
