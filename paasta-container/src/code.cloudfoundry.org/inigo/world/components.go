package world

import (
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

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	sshproxyconfig "code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/garden"
	gardenclient "code.cloudfoundry.org/garden/client"
	gardenconnection "code.cloudfoundry.org/garden/client/connection"
	gorouterconfig "code.cloudfoundry.org/gorouter/config"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/inigo/gardenrunner"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/volman"
	volmanclient "code.cloudfoundry.org/volman/vollocal"
	"github.com/cloudfoundry-incubator/candiedyaml"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/crypto/ssh"
)

type BuiltExecutables map[string]string
type BuiltLifecycles map[string]string

const (
	LifecycleFilename = "some-lifecycle.tar.gz"
)

type BuiltArtifacts struct {
	Executables BuiltExecutables
	Lifecycles  BuiltLifecycles
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

type ComponentAddresses struct {
	NATS                string
	Etcd                string
	EtcdPeer            string
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
	SQL                 string
}

type ComponentMaker struct {
	Artifacts BuiltArtifacts
	Addresses ComponentAddresses

	ExternalAddress string

	PreloadedStackPathMap map[string]string

	GardenBinPath   string
	GardenGraphPath string

	SSHConfig SSHKeys
	EtcdSSL   SSLConfig
	BbsSSL    SSLConfig
	RepSSL    SSLConfig

	VolmanDriverConfigDir string

	DBDriverName           string
	DBBaseConnectionString string
	UseSQL                 bool
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
	if !maker.UseSQL {
		// If we aren't using SQL, return a no-op ifrit runner
		return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals

			return nil
		})
	}

	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer ginkgo.GinkgoRecover()

		db, err := sql.Open(maker.DBDriverName, maker.DBBaseConnectionString)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()

		Eventually(db.Ping).ShouldNot(HaveOccurred())

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE diego_%d", ginkgo.GinkgoParallelNode()))
		Expect(err).NotTo(HaveOccurred())

		sqlDBName := fmt.Sprintf("diego_%d", ginkgo.GinkgoParallelNode())
		db, err = sql.Open(maker.DBDriverName, fmt.Sprintf("%s%s", maker.DBBaseConnectionString, sqlDBName))
		Expect(err).NotTo(HaveOccurred())
		Eventually(db.Ping).ShouldNot(HaveOccurred())

		Expect(db.Close()).To(Succeed())

		close(ready)

		select {
		case <-signals:
			db, err := sql.Open(maker.DBDriverName, maker.DBBaseConnectionString)
			Expect(err).NotTo(HaveOccurred())
			Eventually(db.Ping).ShouldNot(HaveOccurred())

			_, err = db.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
			Expect(err).NotTo(HaveOccurred())
		}

		return nil
	})
}

func (maker ComponentMaker) Etcd(argv ...string) ifrit.Runner {
	nodeName := fmt.Sprintf("etcd_%d", ginkgo.GinkgoParallelNode())
	dataDir := path.Join(os.TempDir(), nodeName)

	return ginkgomon.New(ginkgomon.Config{
		Name:              "etcd",
		AnsiColorCode:     "31m",
		StartCheck:        "etcdserver: published",
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			"etcd",
			append([]string{
				"--name", nodeName,
				"--data-dir", dataDir,
				"--listen-client-urls", "https://" + maker.Addresses.Etcd,
				"--listen-peer-urls", "http://" + maker.Addresses.EtcdPeer,
				"--initial-cluster", nodeName + "=" + "http://" + maker.Addresses.EtcdPeer,
				"--initial-advertise-peer-urls", "http://" + maker.Addresses.EtcdPeer,
				"--initial-cluster-state", "new",
				"--advertise-client-urls", "https://" + maker.Addresses.Etcd,
				"--cert-file", maker.EtcdSSL.ServerCert,
				"--key-file", maker.EtcdSSL.ServerKey,
				"--ca-file", maker.EtcdSSL.CACert,
			}, argv...)...,
		),
		Cleanup: func() {
			err := os.RemoveAll(dataDir)
			Expect(err).NotTo(HaveOccurred())
		},
	})
}

func (maker ComponentMaker) Consul(argv ...string) ifrit.Runner {
	_, port, err := net.SplitHostPort(maker.Addresses.Consul)
	Expect(err).NotTo(HaveOccurred())
	httpPort, err := strconv.Atoi(port)
	Expect(err).NotTo(HaveOccurred())

	startingPort := httpPort - consulrunner.PortOffsetHTTP

	clusterRunner := consulrunner.NewClusterRunner(startingPort, 1, "http")
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer ginkgo.GinkgoRecover()

		done := make(chan struct{})
		go func() {
			defer ginkgo.GinkgoRecover()
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

func (maker ComponentMaker) Garden(argv ...string) ifrit.Runner {
	gardenArgs := []string{}
	gardenArgs = append(gardenArgs, "--runc-bin", filepath.Join(maker.GardenBinPath, "runc"))
	gardenArgs = append(gardenArgs, "--port-pool-size", "1000")
	gardenArgs = append(gardenArgs, "--allow-host-access", "")
	gardenArgs = append(gardenArgs, "--deny-network", "0.0.0.0/0")
	if gardenrunner.UseOldGardenRunc() {
		gardenArgs = append(gardenArgs, "--iodaemon-bin", maker.GardenBinPath+"/iodaemon")
		gardenArgs = append(gardenArgs, "--kawasaki-bin", maker.GardenBinPath+"/kawasaki")
	}
	return runner.NewGardenRunner(
		maker.Artifacts.Executables["garden"],
		filepath.Join(maker.GardenBinPath, "init"),
		filepath.Join(maker.GardenBinPath, "nstar"),
		filepath.Join(maker.GardenBinPath, "dadoo"),
		filepath.Join(maker.GardenBinPath, "grootfs"),
		maker.PreloadedStackPathMap[maker.DefaultStack()],
		filepath.Join(maker.GardenBinPath, "tar"),
		"tcp",
		maker.Addresses.GardenLinux,
		gardenArgs...,
	)
}

func (maker ComponentMaker) BBS(argv ...string) ifrit.Runner {
	bbsArgs := []string{
		"-activeKeyLabel=" + "secure-key-1",
		"-advertiseURL", maker.BBSURL(),
		"-auctioneerAddress", "http://" + maker.Addresses.Auctioneer,
		"-consulCluster", maker.ConsulCluster(),
		"-encryptionKey=" + "secure-key-1:secure-passphrase",
		"-listenAddress", maker.Addresses.BBS,
		"-healthAddress", maker.Addresses.Health,
		"-logLevel", "debug",
		"-requireSSL",
		"-certFile", maker.BbsSSL.ServerCert,
		"-keyFile", maker.BbsSSL.ServerKey,
		"-caFile", maker.BbsSSL.CACert,
		"-etcdCaFile", maker.EtcdSSL.CACert,
		"-etcdCertFile", maker.EtcdSSL.ClientCert,
		"-etcdCluster", maker.EtcdCluster(),
		"-etcdKeyFile", maker.EtcdSSL.ClientKey,
		"-repCACert", maker.RepSSL.CACert,
		"-repClientCert", maker.RepSSL.ClientCert,
		"-repClientKey", maker.RepSSL.ClientKey,
	}

	if maker.UseSQL {
		bbsArgs = append(bbsArgs,
			"-databaseConnectionString", maker.Addresses.SQL,
			"-databaseDriver", maker.DBDriverName,
		)
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "bbs",
		AnsiColorCode:     "32m",
		StartCheck:        "bbs.started",
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["bbs"],
			append(bbsArgs, argv...)...,
		),
	})
}

func (maker ComponentMaker) Rep(argv ...string) *ginkgomon.Runner {
	return maker.RepN(0, argv...)
}

func (maker ComponentMaker) RepN(n int, argv ...string) *ginkgomon.Runner {
	host, portString, err := net.SplitHostPort(maker.Addresses.Rep)
	Expect(err).NotTo(HaveOccurred())
	port, err := strconv.Atoi(portString)
	Expect(err).NotTo(HaveOccurred())

	name := "rep-" + strconv.Itoa(n)

	tmpDir, err := ioutil.TempDir(os.TempDir(), "executor")
	Expect(err).NotTo(HaveOccurred())

	cachePath := path.Join(tmpDir, "cache")

	args := append(
		[]string{
			"-sessionName", name,
			"-rootFSProvider", "docker",
			"-bbsAddress", maker.BBSURL(),
			"-listenAddr", fmt.Sprintf("%s:%d", host, offsetPort(port, n)),
			"-cellID", "the-cell-id-" + strconv.Itoa(ginkgo.GinkgoParallelNode()) + "-" + strconv.Itoa(n),
			"-pollingInterval", "1s",
			"-evacuationPollingInterval", "1s",
			"-evacuationTimeout", "1s",
			"-lockTTL", "10s",
			"-lockRetryInterval", "1s",
			"-consulCluster", maker.ConsulCluster(),
			"-gardenNetwork", "tcp",
			"-gardenAddr", maker.Addresses.GardenLinux,
			"-containerMaxCpuShares", "1024",
			"-cachePath", cachePath,
			"-tempDir", tmpDir,
			"-logLevel", "debug",
			"-bbsClientCert", maker.BbsSSL.ClientCert,
			"-bbsClientKey", maker.BbsSSL.ClientKey,
			"-bbsCACert", maker.BbsSSL.CACert,
			"-gardenHealthcheckProcessPath", "/bin/sh",
			"-gardenHealthcheckProcessArgs", "-c,echo,foo",
			"-gardenHealthcheckProcessUser", "vcap",
			"-volmanDriverPaths", path.Join(maker.VolmanDriverConfigDir, fmt.Sprintf("node-%d", config.GinkgoConfig.ParallelNode)),
			"-certFile", maker.RepSSL.ServerCert,
			"-keyFile", maker.RepSSL.ServerKey,
			"-caFile", maker.RepSSL.CACert,
			"-requireTLS=true",
			"-enableLegacyApiServer=false",
			"-listenAddrSecurable", fmt.Sprintf("%s:%d", host, offsetPort(port+100, n)),
		},
		argv...,
	)
	for stack, path := range maker.PreloadedStackPathMap {
		args = append(args, "-preloadedRootFS", fmt.Sprintf("%s:%s", stack, path))
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:          name,
		AnsiColorCode: "33m",
		StartCheck:    `"` + name + `.started"`,
		// rep is not started until it can ping an executor and run a healthcheck
		// container on garden; this can take a bit to start, so account for it
		StartCheckTimeout: 2 * time.Minute,
		Command:           exec.Command(maker.Artifacts.Executables["rep"], args...),
		Cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	})
}

func (maker ComponentMaker) Auctioneer(argv ...string) ifrit.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "auctioneer",
		AnsiColorCode:     "35m",
		StartCheck:        `"auctioneer.started"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["auctioneer"],
			append([]string{
				"-bbsAddress", maker.BBSURL(),
				"-listenAddr", maker.Addresses.Auctioneer,
				"-lockRetryInterval", "1s",
				"-consulCluster", maker.ConsulCluster(),
				"-logLevel", "debug",
				"-bbsClientCert", maker.BbsSSL.ClientCert,
				"-bbsClientKey", maker.BbsSSL.ClientKey,
				"-bbsCACert", maker.BbsSSL.CACert,
				"-startingContainerWeight", "0.33",
				"-repCACert", maker.RepSSL.CACert,
				"-repClientCert", maker.RepSSL.ClientCert,
				"-repClientKey", maker.RepSSL.ClientKey,
			}, argv...)...,
		),
	})
}

func (maker ComponentMaker) RouteEmitter(argv ...string) ifrit.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "route-emitter",
		AnsiColorCode:     "36m",
		StartCheck:        `"route-emitter.started"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["route-emitter"],
			append([]string{
				"-natsAddresses", maker.Addresses.NATS,
				"-bbsAddress", maker.BBSURL(),
				"-lockRetryInterval", "1s",
				"-consulCluster", maker.ConsulCluster(),
				"-logLevel", "debug",
				"-bbsClientCert", maker.BbsSSL.ClientCert,
				"-bbsClientKey", maker.BbsSSL.ClientKey,
				"-bbsCACert", maker.BbsSSL.CACert,
			}, argv...)...,
		),
	})
}

func (maker ComponentMaker) FileServer(argv ...string) (ifrit.Runner, string) {
	servedFilesDir, err := ioutil.TempDir("", "file-server-files")
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "file-server",
		AnsiColorCode:     "92m",
		StartCheck:        `"file-server.ready"`,
		StartCheckTimeout: 10 * time.Second,
		Command: exec.Command(
			maker.Artifacts.Executables["file-server"],
			append([]string{
				"-address", maker.Addresses.FileServer,
				"-consulCluster", maker.ConsulCluster(),
				"-logLevel", "debug",
				"-staticDirectory", servedFilesDir,
			}, argv...)...,
		),
		Cleanup: func() {
			err := os.RemoveAll(servedFilesDir)
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

	routerConfig := &gorouterconfig.Config{
		Port: uint16(routerPortInt),

		PruneStaleDropletsInterval: 5 * time.Second,
		DropletStaleThreshold:      10 * time.Second,
		PublishActiveAppsInterval:  0 * time.Second,
		StartResponseDelayInterval: 1 * time.Second,

		Nats: []gorouterconfig.NatsConfig{
			{
				Host: natsHost,
				Port: uint16(natsPortInt),
			},
		},
		Logging: gorouterconfig.LoggingConfig{
			File:          "/dev/stdout",
			Level:         "info",
			MetronAddress: "127.0.0.1:65534", // nonsense to make dropsonde happy
		},
	}

	configFile, err := ioutil.TempFile(os.TempDir(), "router-config")
	Expect(err).NotTo(HaveOccurred())

	defer configFile.Close()

	err = candiedyaml.NewEncoder(configFile).Encode(routerConfig)
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

func (maker ComponentMaker) appendLifecycleArgs(args []string) []string {
	for stack, _ := range maker.PreloadedStackPathMap {
		args = append(args, "-lifecycle", fmt.Sprintf("buildpack/%s:%s", stack, LifecycleFilename))
	}

	return args
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

func (maker ComponentMaker) BBSServiceClient(logger lager.Logger) bbs.ServiceClient {
	client, err := consuladapter.NewClientFromUrl(maker.ConsulCluster())
	Expect(err).NotTo(HaveOccurred())

	return bbs.NewServiceClient(client, clock.NewClock())
}

func (maker ComponentMaker) BBSURL() string {
	return "https://" + maker.Addresses.BBS
}

func (maker ComponentMaker) ConsulCluster() string {
	return "http://" + maker.Addresses.Consul
}

func (maker ComponentMaker) EtcdCluster() string {
	return "https://" + maker.Addresses.Etcd
}

func (maker ComponentMaker) VolmanClient(logger lager.Logger) (volman.Manager, ifrit.Runner) {
	driverConfig := volmanclient.NewDriverConfig()
	driverConfig.DriverPaths = []string{path.Join(maker.VolmanDriverConfigDir, fmt.Sprintf("node-%d", config.GinkgoConfig.ParallelNode))}

	return volmanclient.NewServer(logger, driverConfig)
}

func (maker ComponentMaker) VolmanDriver(logger lager.Logger) (ifrit.Runner, voldriver.Driver) {
	debugServerAddress := fmt.Sprintf("0.0.0.0:%d", 9850+ginkgo.GinkgoParallelNode())
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

// offsetPort retuns a new port offest by a given number in such a way
// that it does not interfere with the ginkgo parallel node offest in the base port.
func offsetPort(basePort, offset int) int {
	return basePort + (10 * offset)
}
