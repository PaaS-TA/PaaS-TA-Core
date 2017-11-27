package world

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"time"

	yaml "gopkg.in/yaml.v2"

	auctioneerconfig "code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbsrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	sshproxyconfig "code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	fileserverconfig "code.cloudfoundry.org/fileserver/cmd/file-server/config"
	"code.cloudfoundry.org/garden"
	gardenclient "code.cloudfoundry.org/garden/client"
	gardenconnection "code.cloudfoundry.org/garden/client/connection"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	"code.cloudfoundry.org/rep/maintain"
	routeemitterconfig "code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	routingapi "code.cloudfoundry.org/route-emitter/cmd/route-emitter/runners"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/volman"
	volmanclient "code.cloudfoundry.org/volman/vollocal"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"
	"golang.org/x/crypto/ssh"
)

type BuiltExecutables map[string]string
type BuiltLifecycles map[string]string

const (
	LifecycleFilename = "lifecycle.tar.gz"
)

type BuiltArtifacts struct {
	Executables BuiltExecutables
	Lifecycles  BuiltLifecycles
	Healthcheck string
}

type SSHKeys struct {
	HostKey       ssh.Signer
	HostKeyPem    string
	PrivateKeyPem string
	AuthorizedKey string
}

type SSLConfig struct {
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
	CACert     string
}

type GardenSettingsConfig struct {
	GrootFSBinPath            string
	GardenBinPath             string
	GardenGraphPath           string
	UnprivilegedGrootfsConfig GrootFSConfig
	PrivilegedGrootfsConfig   GrootFSConfig
}

type GrootFSConfig struct {
	StorePath string `yaml:"store"`
	DraxBin   string `yaml:"drax_bin"`
	LogLevel  string `yaml:"log_level"`
	Create    struct {
		JSON        bool     `yaml:"json"`
		UidMappings []string `yaml:"uid_mappings"`
		GidMappings []string `yaml:"gid_mappings"`
	}
}

type ComponentAddresses struct {
	NATS                string
	Consul              string
	BBS                 string
	Health              string
	Rep                 string
	FileServer          string
	Router              string
	GardenLinux         string
	Auctioneer          string
	SSHProxy            string
	SSHProxyHealthCheck string
	FakeVolmanDriver    string
	Locket              string
	SQL                 string
}

type ComponentMaker struct {
	Artifacts BuiltArtifacts
	Addresses ComponentAddresses

	ExternalAddress string

	PreloadedStackPathMap map[string]string

	GardenConfig GardenSettingsConfig

	SSHConfig     SSHKeys
	BbsSSL        SSLConfig
	LocketSSL     SSLConfig
	RepSSL        SSLConfig
	AuctioneerSSL SSLConfig
	SQLCACertFile string

	VolmanDriverConfigDir string

	DBDriverName           string
	DBBaseConnectionString string
}

func (blc *BuiltLifecycles) BuildLifecycles(lifeCycle string) {
	lifeCyclePath := filepath.Join("code.cloudfoundry.org", lifeCycle)

	builderPath, err := gexec.BuildIn(os.Getenv("APP_LIFECYCLE_GOPATH"), filepath.Join(lifeCyclePath, "builder"), "-race")
	Expect(err).NotTo(HaveOccurred())

	launcherPath, err := gexec.BuildIn(os.Getenv("APP_LIFECYCLE_GOPATH"), filepath.Join(lifeCyclePath, "launcher"), "-race")
	Expect(err).NotTo(HaveOccurred())

	healthcheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck", "-race")
	Expect(err).NotTo(HaveOccurred())

	lifecycleDir, err := ioutil.TempDir("", lifeCycle)
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(builderPath, filepath.Join(lifecycleDir, "builder"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(healthcheckPath, filepath.Join(lifecycleDir, "healthcheck"))

	err = os.Rename(launcherPath, filepath.Join(lifecycleDir, "launcher"))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("tar", "-czf", "lifecycle.tar.gz", "builder", "launcher", "healthcheck")
	cmd.Stderr = GinkgoWriter
	cmd.Stdout = GinkgoWriter
	cmd.Dir = lifecycleDir
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	(*blc)[lifeCycle] = filepath.Join(lifecycleDir, LifecycleFilename)
}

func (maker ComponentMaker) Setup() {
	if UseGrootFS() {
		maker.GrootFSInitStore()
	}
}

func (maker ComponentMaker) Teardown() {
	if UseGrootFS() {
		maker.GrootFSDeleteStore()
	}
}

func (maker ComponentMaker) NATS(argv ...string) ifrit.Runner {
	host, port, err := net.SplitHostPort(maker.Addresses.NATS)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "gnatsd",
		AnsiColorCode:     "30m",
		StartCheck:        "gnatsd is ready",
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			"gnatsd",
			append([]string{
				"--addr", host,
				"--port", port,
			}, argv...)...,
		),
	})
}

func (maker ComponentMaker) SQL(argv ...string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer GinkgoRecover()
		dbConnectionString := appendExtraConnectionStringParam(maker.DBDriverName, maker.DBBaseConnectionString, maker.SQLCACertFile)

		db, err := sql.Open(maker.DBDriverName, dbConnectionString)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		Eventually(db.Ping).Should(Succeed())

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE diego_%d", GinkgoParallelNode()))
		Expect(err).NotTo(HaveOccurred())

		sqlDBName := fmt.Sprintf("diego_%d", GinkgoParallelNode())
		dbWithDatabaseNameConnectionString := appendExtraConnectionStringParam(maker.DBDriverName, fmt.Sprintf("%s%s", maker.DBBaseConnectionString, sqlDBName), maker.SQLCACertFile)
		db, err = sql.Open(maker.DBDriverName, dbWithDatabaseNameConnectionString)
		Expect(err).NotTo(HaveOccurred())
		Eventually(db.Ping).Should(Succeed())

		Expect(db.Close()).To(Succeed())

		close(ready)

		select {
		case <-signals:
			db, err := sql.Open(maker.DBDriverName, dbConnectionString)
			Expect(err).NotTo(HaveOccurred())
			Eventually(db.Ping).ShouldNot(HaveOccurred())

			_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
			Expect(err).NotTo(HaveOccurred())
		}

		return nil
	})
}

func (maker ComponentMaker) Consul(argv ...string) ifrit.Runner {
	_, port, err := net.SplitHostPort(maker.Addresses.Consul)
	Expect(err).NotTo(HaveOccurred())
	httpPort, err := strconv.Atoi(port)
	Expect(err).NotTo(HaveOccurred())

	startingPort := httpPort - consulrunner.PortOffsetHTTP

	clusterRunner := consulrunner.NewClusterRunner(
		consulrunner.ClusterRunnerConfig{
			StartingPort: startingPort,
			NumNodes:     1,
			Scheme:       "http",
		},
	)
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer GinkgoRecover()

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			clusterRunner.Start()
			close(done)
		}()

		Eventually(done, 10).Should(BeClosed())

		close(ready)

		select {
		case <-signals:
			clusterRunner.Stop()
		}

		return nil
	})
}

func (maker ComponentMaker) GrootFSInitStore() {
	err := maker.grootfsInitStore(maker.GardenConfig.UnprivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())

	err = maker.grootfsInitStore(maker.GardenConfig.PrivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())
}

func (maker ComponentMaker) grootfsInitStore(grootfsConfig GrootFSConfig) error {
	grootfsArgs := []string{}
	grootfsArgs = append(grootfsArgs, "--config", maker.grootfsConfigPath(grootfsConfig))
	grootfsArgs = append(grootfsArgs, "init-store")
	for _, mapping := range grootfsConfig.Create.UidMappings {
		grootfsArgs = append(grootfsArgs, "--uid-mapping", mapping)
	}
	for _, mapping := range grootfsConfig.Create.GidMappings {
		grootfsArgs = append(grootfsArgs, "--gid-mapping", mapping)
	}

	return maker.grootfsRunner(grootfsArgs)
}

func (maker ComponentMaker) GrootFSDeleteStore() {
	err := maker.grootfsDeleteStore(maker.GardenConfig.UnprivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())

	err = maker.grootfsDeleteStore(maker.GardenConfig.PrivilegedGrootfsConfig)
	Expect(err).NotTo(HaveOccurred())
}

func (maker ComponentMaker) grootfsDeleteStore(grootfsConfig GrootFSConfig) error {
	grootfsArgs := []string{}
	grootfsArgs = append(grootfsArgs, "--config", maker.grootfsConfigPath(grootfsConfig))
	grootfsArgs = append(grootfsArgs, "delete-store")
	return maker.grootfsRunner(grootfsArgs)
}

func (maker ComponentMaker) grootfsRunner(args []string) error {
	cmd := exec.Command(filepath.Join(maker.GardenConfig.GrootFSBinPath, "grootfs"), args...)
	cmd.Stderr = GinkgoWriter
	cmd.Stdout = GinkgoWriter
	return cmd.Run()
}

func (maker ComponentMaker) grootfsConfigPath(grootfsConfig GrootFSConfig) string {
	configFile, err := ioutil.TempFile("", "grootfs-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()
	data, err := yaml.Marshal(&grootfsConfig)
	Expect(err).NotTo(HaveOccurred())
	_, err = configFile.Write(data)
	Expect(err).NotTo(HaveOccurred())

	return configFile.Name()
}

func (maker ComponentMaker) GardenWithoutDefaultStack() ifrit.Runner {
	return maker.garden(false)
}

func (maker ComponentMaker) Garden() ifrit.Runner {
	return maker.garden(true)
}

func (maker ComponentMaker) garden(includeDefaultStack bool) ifrit.Runner {
	defaultRootFS := ""
	if includeDefaultStack {
		defaultRootFS = maker.PreloadedStackPathMap[maker.DefaultStack()]
	}

	members := []grouper.Member{}

	config := runner.DefaultGdnRunnerConfig()

	config.GdnBin = maker.Artifacts.Executables["garden"]
	config.TarBin = filepath.Join(maker.GardenConfig.GardenBinPath, "tar")
	config.InitBin = filepath.Join(maker.GardenConfig.GardenBinPath, "init")
	config.ExecRunnerBin = filepath.Join(maker.GardenConfig.GardenBinPath, "dadoo")
	config.NSTarBin = filepath.Join(maker.GardenConfig.GardenBinPath, "nstar")
	config.RuntimePluginBin = filepath.Join(maker.GardenConfig.GardenBinPath, "runc")

	config.DefaultRootFS = defaultRootFS

	config.AllowHostAccess = boolPtr(true)

	config.DenyNetworks = []string{"0.0.0.0/0"}

	host, port, err := net.SplitHostPort(maker.Addresses.GardenLinux)
	Expect(err).NotTo(HaveOccurred())

	config.BindSocket = ""
	config.BindIP = host

	intPort, err := strconv.Atoi(port)
	Expect(err).NotTo(HaveOccurred())
	config.BindPort = intPtr(intPort)

	if UseGrootFS() {
		config.ImagePluginBin = filepath.Join(maker.GardenConfig.GrootFSBinPath, "grootfs")
		config.PrivilegedImagePluginBin = filepath.Join(maker.GardenConfig.GrootFSBinPath, "grootfs")

		config.ImagePluginExtraArgs = []string{
			"\"--config\"",
			maker.grootfsConfigPath(maker.GardenConfig.UnprivilegedGrootfsConfig),
		}

		config.PrivilegedImagePluginExtraArgs = []string{
			"\"--config\"",
			maker.grootfsConfigPath(maker.GardenConfig.PrivilegedGrootfsConfig),
		}
	}

	gardenRunner := runner.NewGardenRunner(config)

	members = append(members, grouper.Member{"garden", gardenRunner})

	return grouper.NewOrdered(os.Interrupt, members)
}

func (maker ComponentMaker) RoutingAPI(modifyConfigFuncs ...func(*routingapi.Config)) *routingapi.RoutingAPIRunner {
	binPath := maker.Artifacts.Executables["routing-api"]

	sqlConfig := routingapi.SQLConfig{
		DriverName: maker.DBDriverName,
		DBName:     fmt.Sprintf("routingapi_%d", GinkgoParallelNode()),
	}

	if maker.DBDriverName == "mysql" {
		sqlConfig.Port = 3306
		sqlConfig.Username = "diego"
		sqlConfig.Password = "diego_password"
	} else {
		sqlConfig.Port = 5432
		sqlConfig.Username = "diego"
		sqlConfig.Password = "diego_pw"
	}

	runner, err := routingapi.NewRoutingAPIRunner(binPath, maker.ConsulCluster(), sqlConfig, modifyConfigFuncs...)
	Expect(err).NotTo(HaveOccurred())
	return runner
}

func (maker ComponentMaker) BBS(modifyConfigFuncs ...func(*bbsconfig.BBSConfig)) ifrit.Runner {
	config := bbsconfig.BBSConfig{
		AdvertiseURL:  maker.BBSURL(),
		ConsulCluster: maker.ConsulCluster(),
		EncryptionConfig: encryption.EncryptionConfig{
			ActiveKeyLabel: "secure-key-1",
			EncryptionKeys: map[string]string{
				"secure-key-1": "secure-passphrase",
			},
		},
		LagerConfig: lagerflags.LagerConfig{
			LogLevel: "debug",
		},
		AuctioneerAddress:             "https://" + maker.Addresses.Auctioneer,
		ListenAddress:                 maker.Addresses.BBS,
		HealthAddress:                 maker.Addresses.Health,
		RequireSSL:                    true,
		CertFile:                      maker.BbsSSL.ServerCert,
		KeyFile:                       maker.BbsSSL.ServerKey,
		CaFile:                        maker.BbsSSL.CACert,
		RepCACert:                     maker.RepSSL.CACert,
		RepClientCert:                 maker.RepSSL.ClientCert,
		RepClientKey:                  maker.RepSSL.ClientKey,
		AuctioneerCACert:              maker.AuctioneerSSL.CACert,
		AuctioneerClientCert:          maker.AuctioneerSSL.ClientCert,
		AuctioneerClientKey:           maker.AuctioneerSSL.ClientKey,
		DatabaseConnectionString:      maker.Addresses.SQL,
		DatabaseDriver:                maker.DBDriverName,
		DetectConsulCellRegistrations: true,
		AuctioneerRequireTLS:          true,
		SQLCACertFile:                 maker.SQLCACertFile,
		ClientLocketConfig:            maker.locketClientConfig(),
		UUID:                          "bbs-inigo-lock-owner",
	}

	for _, modifyConfig := range modifyConfigFuncs {
		modifyConfig(&config)
	}

	runner := bbsrunner.New(maker.Artifacts.Executables["bbs"], config)
	runner.AnsiColorCode = "32m"
	runner.StartCheckTimeout = 10 * time.Second
	return runner
}

func (maker ComponentMaker) Locket(modifyConfigFuncs ...func(*locketconfig.LocketConfig)) ifrit.Runner {
	return locketrunner.NewLocketRunner(maker.Artifacts.Executables["locket"], func(cfg *locketconfig.LocketConfig) {
		cfg.CertFile = maker.LocketSSL.ServerCert
		cfg.KeyFile = maker.LocketSSL.ServerKey
		cfg.CaFile = maker.LocketSSL.CACert
		cfg.ConsulCluster = maker.ConsulCluster()
		cfg.DatabaseConnectionString = maker.Addresses.SQL
		cfg.DatabaseDriver = maker.DBDriverName
		cfg.ListenAddress = maker.Addresses.Locket
		cfg.SQLCACertFile = maker.SQLCACertFile
		cfg.LagerConfig = lagerflags.LagerConfig{
			LogLevel: "debug",
		}

		for _, modifyConfig := range modifyConfigFuncs {
			modifyConfig(cfg)
		}
	})
}

func (maker ComponentMaker) Rep(modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	return maker.RepN(0, modifyConfigFuncs...)
}

func (maker ComponentMaker) RepN(n int, modifyConfigFuncs ...func(*repconfig.RepConfig)) *ginkgomon.Runner {
	host, portString, err := net.SplitHostPort(maker.Addresses.Rep)
	Expect(err).NotTo(HaveOccurred())
	port, err := strconv.Atoi(portString)
	Expect(err).NotTo(HaveOccurred())

	name := "rep-" + strconv.Itoa(n)

	tmpDir, err := ioutil.TempDir(os.TempDir(), "executor")
	Expect(err).NotTo(HaveOccurred())

	cachePath := path.Join(tmpDir, "cache")

	repConfig := repconfig.RepConfig{
		SessionName:               name,
		SupportedProviders:        []string{"docker"},
		BBSAddress:                maker.BBSURL(),
		ListenAddr:                fmt.Sprintf("%s:%d", host, offsetPort(port, n)),
		CellID:                    "the-cell-id-" + strconv.Itoa(GinkgoParallelNode()) + "-" + strconv.Itoa(n),
		PollingInterval:           durationjson.Duration(1 * time.Second),
		EvacuationPollingInterval: durationjson.Duration(1 * time.Second),
		EvacuationTimeout:         durationjson.Duration(1 * time.Second),
		LockTTL:                   durationjson.Duration(10 * time.Second),
		LockRetryInterval:         durationjson.Duration(1 * time.Second),
		ConsulCluster:             maker.ConsulCluster(),
		BBSClientCertFile:         maker.BbsSSL.ClientCert,
		BBSClientKeyFile:          maker.BbsSSL.ClientKey,
		BBSCACertFile:             maker.BbsSSL.CACert,
		ServerCertFile:            maker.RepSSL.ServerCert,
		ServerKeyFile:             maker.RepSSL.ServerKey,
		CaCertFile:                maker.RepSSL.CACert,
		RequireTLS:                true,
		EnableLegacyAPIServer:     false,
		ListenAddrSecurable:       fmt.Sprintf("%s:%d", host, offsetPort(port+100, n)),
		PreloadedRootFS:           maker.PreloadedStackPathMap,
		ExecutorConfig: executorinit.ExecutorConfig{
			GardenNetwork:         "tcp",
			GardenAddr:            maker.Addresses.GardenLinux,
			ContainerMaxCpuShares: 1024,
			CachePath:             cachePath,
			TempDir:               tmpDir,
			GardenHealthcheckProcessPath:  "/bin/sh",
			GardenHealthcheckProcessArgs:  []string{"-c", "echo", "foo"},
			GardenHealthcheckProcessUser:  "vcap",
			VolmanDriverPaths:             path.Join(maker.VolmanDriverConfigDir, fmt.Sprintf("node-%d", config.GinkgoConfig.ParallelNode)),
			ContainerOwnerName:            "executor-" + strconv.Itoa(n),
			HealthCheckContainerOwnerName: "executor-health-check-" + strconv.Itoa(n),
		},
		LagerConfig: lagerflags.LagerConfig{
			LogLevel: "debug",
		},
	}

	for _, modifyConfig := range modifyConfigFuncs {
		modifyConfig(&repConfig)
	}

	configFile, err := ioutil.TempFile(os.TempDir(), "rep-config")
	Expect(err).NotTo(HaveOccurred())

	defer configFile.Close()

	err = json.NewEncoder(configFile).Encode(repConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:          name,
		AnsiColorCode: "33m",
		StartCheck:    `"` + name + `.started"`,
		// rep is not started until it can ping an executor and run a healthcheck
		// container on garden; this can take a bit to start, so account for it
		StartCheckTimeout: 2 * time.Minute,
		Command: exec.Command(
			maker.Artifacts.Executables["rep"],
			"-config", configFile.Name()),
		Cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	})
}

func (maker ComponentMaker) Auctioneer() ifrit.Runner {
	auctioneerConfig := auctioneerconfig.AuctioneerConfig{
		BBSAddress:              maker.BBSURL(),
		ListenAddress:           maker.Addresses.Auctioneer,
		LockRetryInterval:       durationjson.Duration(time.Second),
		ConsulCluster:           maker.ConsulCluster(),
		BBSClientCertFile:       maker.BbsSSL.ClientCert,
		BBSClientKeyFile:        maker.BbsSSL.ClientKey,
		BBSCACertFile:           maker.BbsSSL.CACert,
		StartingContainerWeight: 0.33,
		RepCACert:               maker.RepSSL.CACert,
		RepClientCert:           maker.RepSSL.ClientCert,
		RepClientKey:            maker.RepSSL.ClientKey,
		CACertFile:              maker.AuctioneerSSL.CACert,
		ServerCertFile:          maker.AuctioneerSSL.ServerCert,
		ServerKeyFile:           maker.AuctioneerSSL.ServerKey,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel: "debug",
		},
		ClientLocketConfig: maker.locketClientConfig(),
		UUID:               "auctioneer-inigo-lock-owner",
	}

	configFile, err := ioutil.TempFile(os.TempDir(), "auctioneer-config")
	Expect(err).NotTo(HaveOccurred())

	err = json.NewEncoder(configFile).Encode(auctioneerConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "auctioneer",
		AnsiColorCode:     "35m",
		StartCheck:        `"auctioneer.started"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["auctioneer"],
			"-config", configFile.Name(),
		),
	})
}

func (maker ComponentMaker) RouteEmitter() ifrit.Runner {
	return maker.RouteEmitterN(0, func(*routeemitterconfig.RouteEmitterConfig) {})
}

func (maker ComponentMaker) RouteEmitterN(n int, fs ...func(config *routeemitterconfig.RouteEmitterConfig)) ifrit.Runner {
	name := "route-emitter-" + strconv.Itoa(n)

	configFile, err := ioutil.TempFile("", "file-server-config")
	Expect(err).NotTo(HaveOccurred())

	cfg := routeemitterconfig.RouteEmitterConfig{
		ConsulSessionName: name,
		NATSAddresses:     maker.Addresses.NATS,
		BBSAddress:        maker.BBSURL(),
		LockRetryInterval: durationjson.Duration(time.Second),
		ConsulCluster:     maker.ConsulCluster(),
		LagerConfig:       lagerflags.LagerConfig{LogLevel: "debug"},
		BBSClientCertFile: maker.BbsSSL.ClientCert,
		BBSClientKeyFile:  maker.BbsSSL.ClientKey,
		BBSCACertFile:     maker.BbsSSL.CACert,
	}

	for _, f := range fs {
		f(&cfg)
	}

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&cfg)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              name,
		AnsiColorCode:     "36m",
		StartCheck:        `"` + name + `.watcher.sync.complete"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["route-emitter"],
			"-config", configFile.Name(),
		),
		Cleanup: func() {
			configFile.Close()
			os.RemoveAll(configFile.Name())
		},
	})
}

func (maker ComponentMaker) FileServer() (ifrit.Runner, string) {
	servedFilesDir, err := ioutil.TempDir("", "file-server-files")
	Expect(err).NotTo(HaveOccurred())

	configFile, err := ioutil.TempFile("", "file-server-config")
	Expect(err).NotTo(HaveOccurred())

	cfg := fileserverconfig.FileServerConfig{
		ServerAddress:   maker.Addresses.FileServer,
		ConsulCluster:   maker.ConsulCluster(),
		LagerConfig:     lagerflags.LagerConfig{LogLevel: "debug"},
		StaticDirectory: servedFilesDir,
	}

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&cfg)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "file-server",
		AnsiColorCode:     "92m",
		StartCheck:        `"file-server.ready"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["file-server"],
			"-config", configFile.Name(),
		),
		Cleanup: func() {
			err := os.RemoveAll(servedFilesDir)
			Expect(err).NotTo(HaveOccurred())
			configFile.Close()
			err = os.RemoveAll(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
		},
	}), servedFilesDir
}

func (maker ComponentMaker) Router() ifrit.Runner {
	_, routerPort, err := net.SplitHostPort(maker.Addresses.Router)
	Expect(err).NotTo(HaveOccurred())

	routerPortInt, err := strconv.Atoi(routerPort)
	Expect(err).NotTo(HaveOccurred())

	natsHost, natsPort, err := net.SplitHostPort(maker.Addresses.NATS)
	Expect(err).NotTo(HaveOccurred())

	natsPortInt, err := strconv.Atoi(natsPort)
	Expect(err).NotTo(HaveOccurred())

	routerConfig := `
status:
  port: 0
  user: ""
  pass: ""
nats:
- host: %s
  port: %d
  user: ""
  pass: ""
logging:
  file: /dev/stdout
  syslog: ""
  level: info
  loggregator_enabled: false
  metron_address: 127.0.0.1:65534
port: %d
index: 0
zone: ""
tracing:
  enable_zipkin: false
trace_key: ""
access_log:
  file: ""
  enable_streaming: false
enable_access_log_streaming: false
debug_addr: ""
enable_proxy: false
enable_ssl: false
ssl_port: 0
ssl_cert_path: ""
ssl_key_path: ""
sslcertificate:
  certificate: []
  privatekey: null
  ocspstaple: []
  signedcertificatetimestamps: []
  leaf: null
skip_ssl_validation: false
cipher_suites: ""
ciphersuites: []
load_balancer_healthy_threshold: 0s
publish_start_message_interval: 0s
suspend_pruning_if_nats_unavailable: false
prune_stale_droplets_interval: 5s
droplet_stale_threshold: 10s
publish_active_apps_interval: 0s
start_response_delay_interval: 1s
endpoint_timeout: 0s
route_services_timeout: 0s
secure_cookies: false
oauth:
  token_endpoint: ""
  port: 0
  skip_ssl_validation: false
  client_name: ""
  client_secret: ""
  ca_certs: ""
routing_api:
  uri: ""
  port: 0
  auth_disabled: false
route_services_secret: ""
route_services_secret_decrypt_only: ""
route_services_recommend_https: false
extra_headers_to_log: []
token_fetcher_max_retries: 0
token_fetcher_retry_interval: 0s
token_fetcher_expiration_buffer_time: 0
pid_file: ""
`
	routerConfig = fmt.Sprintf(routerConfig, natsHost, uint16(natsPortInt), uint16(routerPortInt))

	configFile, err := ioutil.TempFile(os.TempDir(), "router-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()
	_, err = configFile.Write([]byte(routerConfig))
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "router",
		AnsiColorCode:     "93m",
		StartCheck:        "router.started",
		StartCheckTimeout: 10 * time.Second, // it waits 1 second before listening. yep.
		Command: exec.Command(
			maker.Artifacts.Executables["router"],
			"-c", configFile.Name(),
		),
		Cleanup: func() {
			err := os.Remove(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
		},
	})
}

func (maker ComponentMaker) SSHProxy(argv ...string) ifrit.Runner {
	sshProxyConfig := sshproxyconfig.SSHProxyConfig{
		Address:            maker.Addresses.SSHProxy,
		HealthCheckAddress: maker.Addresses.SSHProxyHealthCheck,
		BBSAddress:         maker.BBSURL(),
		BBSCACert:          maker.BbsSSL.CACert,
		BBSClientCert:      maker.BbsSSL.ClientCert,
		BBSClientKey:       maker.BbsSSL.ClientKey,
		ConsulCluster:      maker.ConsulCluster(),
		EnableDiegoAuth:    true,
		HostKey:            maker.SSHConfig.HostKeyPem,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel: "debug",
		},
	}

	configFile, err := ioutil.TempFile("", "ssh-proxy-config")
	Expect(err).NotTo(HaveOccurred())
	defer configFile.Close()

	encoder := json.NewEncoder(configFile)
	err = encoder.Encode(&sshProxyConfig)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "ssh-proxy",
		AnsiColorCode:     "96m",
		StartCheck:        "ssh-proxy.started",
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["ssh-proxy"],
			append([]string{
				"-config", configFile.Name(),
			}, argv...)...,
		),
	})
}

func (maker ComponentMaker) DefaultStack() string {
	Expect(maker.PreloadedStackPathMap).NotTo(BeEmpty())

	var defaultStack string
	for stack, _ := range maker.PreloadedStackPathMap {
		defaultStack = stack
		break
	}

	return defaultStack
}

func (maker ComponentMaker) GardenClient() garden.Client {
	return gardenclient.New(gardenconnection.New("tcp", maker.Addresses.GardenLinux))
}

func (maker ComponentMaker) BBSClient() bbs.InternalClient {
	client, err := bbs.NewSecureClient(
		maker.BBSURL(),
		maker.BbsSSL.CACert,
		maker.BbsSSL.ClientCert,
		maker.BbsSSL.ClientKey,
		0, 0,
	)
	Expect(err).NotTo(HaveOccurred())
	return client
}

func (maker ComponentMaker) BBSServiceClient(logger lager.Logger) serviceclient.ServiceClient {
	client, err := consuladapter.NewClientFromUrl(maker.ConsulCluster())
	Expect(err).NotTo(HaveOccurred())

	cellPresenceClient := maintain.NewCellPresenceClient(client, clock.NewClock())
	locketClient := serviceclient.NewNoopLocketClient()

	return serviceclient.NewServiceClient(cellPresenceClient, locketClient)
}

func (maker ComponentMaker) BBSURL() string {
	return "https://" + maker.Addresses.BBS
}

func (maker ComponentMaker) ConsulCluster() string {
	return "http://" + maker.Addresses.Consul
}

func (maker ComponentMaker) VolmanClient(logger lager.Logger) (volman.Manager, ifrit.Runner) {
	driverConfig := volmanclient.NewDriverConfig()
	driverConfig.DriverPaths = []string{path.Join(maker.VolmanDriverConfigDir, fmt.Sprintf("node-%d", config.GinkgoConfig.ParallelNode))}

	metronClient, err := loggregator_v2.NewIngressClient(loggregator_v2.Config{})
	Expect(err).NotTo(HaveOccurred())
	return volmanclient.NewServer(logger, metronClient, driverConfig)
}

func (maker ComponentMaker) VolmanDriver(logger lager.Logger) (ifrit.Runner, voldriver.Driver) {
	debugServerAddress := fmt.Sprintf("0.0.0.0:%d", 9850+GinkgoParallelNode())
	fakeDriverRunner := ginkgomon.New(ginkgomon.Config{
		Name: "local-driver",
		Command: exec.Command(
			maker.Artifacts.Executables["local-driver"],
			"-listenAddr", maker.Addresses.FakeVolmanDriver,
			"-debugAddr", debugServerAddress,
			"-mountDir", maker.VolmanDriverConfigDir,
			"-driversPath", path.Join(maker.VolmanDriverConfigDir, fmt.Sprintf("node-%d", config.GinkgoConfig.ParallelNode)),
		),
		StartCheck: "local-driver-server.started",
	})

	client, err := driverhttp.NewRemoteClient("http://"+maker.Addresses.FakeVolmanDriver, nil)
	Expect(err).NotTo(HaveOccurred())

	return fakeDriverRunner, client
}

func (maker ComponentMaker) locketClientConfig() locket.ClientLocketConfig {
	return locket.ClientLocketConfig{
		LocketAddress:        maker.Addresses.Locket,
		LocketCACertFile:     maker.LocketSSL.CACert,
		LocketClientCertFile: maker.LocketSSL.ClientCert,
		LocketClientKeyFile:  maker.LocketSSL.ClientKey,
	}
}

// offsetPort retuns a new port offest by a given number in such a way
// that it does not interfere with the ginkgo parallel node offest in the base port.
func offsetPort(basePort, offset int) int {
	return basePort + (10 * offset)
}

func appendExtraConnectionStringParam(driverName, databaseConnectionString, sqlCACertFile string) string {
	switch driverName {
	case "mysql":
		cfg, err := mysql.ParseDSN(databaseConnectionString)
		Expect(err).NotTo(HaveOccurred())

		if sqlCACertFile != "" {
			certBytes, err := ioutil.ReadFile(sqlCACertFile)
			Expect(err).NotTo(HaveOccurred())

			caCertPool := x509.NewCertPool()
			Expect(caCertPool.AppendCertsFromPEM(certBytes)).To(BeTrue())

			tlsConfig := &tls.Config{
				InsecureSkipVerify: false,
				RootCAs:            caCertPool,
			}

			mysql.RegisterTLSConfig("bbs-tls", tlsConfig)
			cfg.TLSConfig = "bbs-tls"
		}
		cfg.Timeout = 10 * time.Minute
		cfg.ReadTimeout = 10 * time.Minute
		cfg.WriteTimeout = 10 * time.Minute
		databaseConnectionString = cfg.FormatDSN()
	case "postgres":
		var err error
		databaseConnectionString, err = pq.ParseURL(databaseConnectionString)
		Expect(err).NotTo(HaveOccurred())
		if sqlCACertFile == "" {
			databaseConnectionString = databaseConnectionString + " sslmode=disable"
		} else {
			databaseConnectionString = fmt.Sprintf("%s sslmode=verify-ca sslrootcert=%s", databaseConnectionString, sqlCACertFile)
		}
	}

	return databaseConnectionString
}

func UseGrootFS() bool {
	return os.Getenv("USE_GROOTFS") == "true"
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
