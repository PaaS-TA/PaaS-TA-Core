package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/agent"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/chaperon"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/status"
	"github.com/cloudfoundry-incubator/consul-release/src/confab/utils"
	"github.com/hashicorp/consul/api"
)

type runner interface {
	Start(config.Config, utils.Timeout) error
	Stop()
}

type stringSlice []string

func (ss *stringSlice) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *stringSlice) Set(value string) error {
	*ss = append(*ss, value)

	return nil
}

var (
	recursors            stringSlice
	configFile           string
	configConsulLinkFile string
	foreground           bool

	stdout = log.New(os.Stdout, "", 0)
	stderr = log.New(os.Stderr, "", 0)
)

func main() {
	flagSet := flag.NewFlagSet("flags", flag.ContinueOnError)
	flagSet.Var(&recursors, "recursor", "specifies the address of an upstream DNS `server`, may be specified multiple times")
	flagSet.StringVar(&configFile, "config-file", "", "specifies the config `file`")
	flagSet.StringVar(&configConsulLinkFile, "config-consul-link-file", "", "specifies the consul link config `file`")
	flagSet.BoolVar(&foreground, "foreground", false, "if true confab will wait for consul to exit")

	if len(os.Args) < 2 {
		printUsageAndExit("invalid number of arguments", flagSet)
	}

	if err := flagSet.Parse(os.Args[2:]); err != nil {
		os.Exit(1)
	}

	configFileContents, err := ioutil.ReadFile(configFile)
	if err != nil {
		stderr.Printf("error reading configuration file: %s", err)
		os.Exit(1)
	}

	configConsulLinkFileContents, err := ioutil.ReadFile(configConsulLinkFile)
	if err != nil {
		stderr.Printf("error reading configuration file: %s", err)
		os.Exit(1)
	}

	cfg, err := config.ConfigFromJSON(configFileContents, configConsulLinkFileContents)
	if err != nil {
		stderr.Printf("error reading configuration file: %s", err)
		os.Exit(1)
	}

	path, err := exec.LookPath(cfg.Path.AgentPath)
	if err != nil {
		printUsageAndExit(fmt.Sprintf("\"agent_path\" %q cannot be found", cfg.Path.AgentPath), flagSet)
	}

	if len(cfg.Path.PIDFile) == 0 {
		printUsageAndExit("\"pid_file\" cannot be empty", flagSet)
	}

	logger := lager.NewLogger("confab")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	agentRunner := &agent.Runner{
		Path:      path,
		PIDFile:   cfg.Path.PIDFile,
		ConfigDir: cfg.Path.ConsulConfigDir,
		Recursors: recursors,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Logger:    logger,
	}

	clientConfig := api.DefaultConfig()
	if cfg.Consul.Agent.RequireSSL {
		clientConfig.Scheme = "https"
		certsDir := filepath.Join(cfg.Path.ConsulConfigDir, "certs")
		tlsConfig := api.TLSConfig{
			Address:  clientConfig.Address,
			CAFile:   filepath.Join(certsDir, "ca.crt"),
			CertFile: filepath.Join(certsDir, "agent.crt"),
			KeyFile:  filepath.Join(certsDir, "agent.key"),
		}
		tlsClientConfig, err := api.SetupTLSConfig(&tlsConfig)
		if err != nil {
			stderr.Printf("error setting up TLS config: %s", err)
			os.Exit(1)
		}
		if transport, ok := clientConfig.HttpClient.Transport.(*http.Transport); ok {
			transport.TLSClientConfig = tlsClientConfig
		}
	}

	consulAPIClient, err := api.NewClient(clientConfig)
	if err != nil {
		panic(err) // not tested, NewClient never errors
	}

	agentClient := &agent.Client{
		ExpectedMembers:   cfg.Consul.Agent.Servers.LAN,
		ConsulAPIAgent:    consulAPIClient.Agent(),
		ConsulAPIOperator: consulAPIClient.Operator(),
		Logger:            logger,
	}

	retrier := utils.NewRetrier(clock.NewClock(), 1*time.Second)

	controller := chaperon.Controller{
		AgentRunner:    agentRunner,
		AgentClient:    agentClient,
		Retrier:        retrier,
		EncryptKeys:    cfg.Consul.EncryptKeys,
		Logger:         logger,
		ServiceDefiner: config.ServiceDefiner{logger},
		ConfigDir:      cfg.Path.ConsulConfigDir,
		Config:         cfg,
	}

	keyringRemover := chaperon.NewKeyringRemover(cfg.Path.KeyringFile, logger)
	configWriter := chaperon.NewConfigWriter(cfg.Path.ConsulConfigDir, logger)

	var r runner = chaperon.NewClient(controller, keyringRemover, configWriter)
	if controller.Config.Consul.Agent.Mode == "server" {
		bootstrapChecker := chaperon.NewBootstrapChecker(logger, agentClient, status.Client{ConsulAPIStatus: consulAPIClient.Status()}, time.Sleep)
		r = chaperon.NewServer(controller, configWriter, bootstrapChecker)
	}

	switch os.Args[1] {
	case "start":
		_, err = os.Stat(controller.Config.Path.ConsulConfigDir)
		if err != nil {
			printUsageAndExit(fmt.Sprintf("\"consul_config_dir\" %q could not be found",
				controller.Config.Path.ConsulConfigDir), flagSet)
		}

		if utils.IsRunningProcess(agentRunner.PIDFile) {
			stderr.Println("consul_agent is already running, please stop it first")
			os.Exit(1)
		}

		if len(agentClient.ExpectedMembers) == 0 {
			printUsageAndExit("at least one \"expected-member\" must be provided", flagSet)
		}

		timeout := utils.NewTimeout(time.After(time.Duration(controller.Config.Confab.TimeoutInSeconds) * time.Second))

		if err := r.Start(cfg, timeout); err != nil {
			stderr.Printf("error during start: %s", err)
			r.Stop()
			os.Exit(1)
		}
		if foreground {
			if err := agentRunner.Wait(); err != nil {
				stderr.Printf("error during wait: %s", err)
				r.Stop()
				os.Exit(1)
			}
		}
	case "stop":
		r.Stop()
	default:
		printUsageAndExit(fmt.Sprintf("invalid COMMAND %q", os.Args[1]), flagSet)
	}
}

func printUsageAndExit(message string, flagSet *flag.FlagSet) {
	stderr.Printf("%s\n\n", message)
	stderr.Println("usage: confab COMMAND OPTIONS\n")
	stderr.Println("COMMAND: \"start\" or \"stop\"")
	stderr.Println("\nOPTIONS:")
	flagSet.PrintDefaults()
	stderr.Println()
	os.Exit(1)
}
