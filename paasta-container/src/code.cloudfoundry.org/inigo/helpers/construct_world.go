package helpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/inigo/world"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/gomega"
)

var PreloadedStacks = []string{"red-stack", "blue-stack"}
var DefaultStack = PreloadedStacks[0]

var addresses world.ComponentAddresses

const assetsPath = "../fixtures/certs/"

func MakeComponentMaker(builtArtifacts world.BuiltArtifacts, localIP string) world.ComponentMaker {
	grootfsBinPath := os.Getenv("GROOTFS_BINPATH")
	gardenBinPath := os.Getenv("GARDEN_BINPATH")
	gardenRootFSPath := os.Getenv("GARDEN_ROOTFS")
	gardenGraphPath := os.Getenv("GARDEN_GRAPH_PATH")
	externalAddress := os.Getenv("EXTERNAL_ADDRESS")

	var dbBaseConnectionString string
	var dbDriverName string

	if test_helpers.UsePostgres() {
		dbDriverName = "postgres"
		dbBaseConnectionString = "postgres://diego:diego_pw@127.0.0.1/"
	} else {
		dbDriverName = "mysql"
		dbBaseConnectionString = "diego:diego_password@tcp(localhost:3306)/"
	}

	if gardenGraphPath == "" {
		gardenGraphPath = os.TempDir()
	}

	if world.UseGrootFS() {
		Expect(grootfsBinPath).NotTo(BeEmpty(), "must provide $GROOTFS_BINPATH")
	}
	Expect(gardenBinPath).NotTo(BeEmpty(), "must provide $GARDEN_BINPATH")
	Expect(gardenRootFSPath).NotTo(BeEmpty(), "must provide $GARDEN_ROOTFS")
	Expect(externalAddress).NotTo(BeEmpty(), "must provide $EXTERNAL_ADDRESS")

	stackPathMap := map[string]string{}
	for _, stack := range PreloadedStacks {
		stackPathMap[stack] = gardenRootFSPath
	}

	addresses = world.ComponentAddresses{
		GardenLinux:         fmt.Sprintf("127.0.0.1:%d", 10000+config.GinkgoConfig.ParallelNode),
		NATS:                fmt.Sprintf("127.0.0.1:%d", 11000+config.GinkgoConfig.ParallelNode),
		Consul:              fmt.Sprintf("127.0.0.1:%d", 12750+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength),
		Rep:                 fmt.Sprintf("127.0.0.1:%d", 14000+config.GinkgoConfig.ParallelNode),
		FileServer:          fmt.Sprintf("%s:%d", localIP, 17000+config.GinkgoConfig.ParallelNode),
		Router:              fmt.Sprintf("127.0.0.1:%d", 18000+config.GinkgoConfig.ParallelNode),
		BBS:                 fmt.Sprintf("127.0.0.1:%d", 20500+config.GinkgoConfig.ParallelNode*2),
		Health:              fmt.Sprintf("127.0.0.1:%d", 20500+config.GinkgoConfig.ParallelNode*2+1),
		Auctioneer:          fmt.Sprintf("127.0.0.1:%d", 23000+config.GinkgoConfig.ParallelNode),
		SSHProxy:            fmt.Sprintf("127.0.0.1:%d", 23500+config.GinkgoConfig.ParallelNode),
		SSHProxyHealthCheck: fmt.Sprintf("127.0.0.1:%d", 24500+config.GinkgoConfig.ParallelNode),
		FakeVolmanDriver:    fmt.Sprintf("127.0.0.1:%d", 25500+config.GinkgoConfig.ParallelNode),
		Locket:              fmt.Sprintf("127.0.0.1:%d", 26500+config.GinkgoConfig.ParallelNode),
		SQL:                 fmt.Sprintf("%sdiego_%d", dbBaseConnectionString, config.GinkgoConfig.ParallelNode),
	}

	hostKeyPair, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	userKeyPair, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	sshKeys := world.SSHKeys{
		HostKey:       hostKeyPair.PrivateKey(),
		HostKeyPem:    hostKeyPair.PEMEncodedPrivateKey(),
		PrivateKeyPem: userKeyPair.PEMEncodedPrivateKey(),
		AuthorizedKey: userKeyPair.AuthorizedKey(),
	}
	bbsServerCert, err := filepath.Abs(assetsPath + "bbs_server.crt")
	Expect(err).NotTo(HaveOccurred())
	bbsServerKey, err := filepath.Abs(assetsPath + "bbs_server.key")
	Expect(err).NotTo(HaveOccurred())
	repServerCert, err := filepath.Abs(assetsPath + "rep_server.crt")
	Expect(err).NotTo(HaveOccurred())
	repServerKey, err := filepath.Abs(assetsPath + "rep_server.key")
	Expect(err).NotTo(HaveOccurred())
	auctioneerServerCert, err := filepath.Abs(assetsPath + "auctioneer_server.crt")
	Expect(err).NotTo(HaveOccurred())
	auctioneerServerKey, err := filepath.Abs(assetsPath + "auctioneer_server.key")
	Expect(err).NotTo(HaveOccurred())
	clientCrt, err := filepath.Abs(assetsPath + "client.crt")
	Expect(err).NotTo(HaveOccurred())
	clientKey, err := filepath.Abs(assetsPath + "client.key")
	Expect(err).NotTo(HaveOccurred())
	caCert, err := filepath.Abs(assetsPath + "ca.crt")
	Expect(err).NotTo(HaveOccurred())

	sqlCACert, err := filepath.Abs(assetsPath + "sql-certs/server-ca.crt")
	Expect(err).NotTo(HaveOccurred())

	sslConfig := world.SSLConfig{
		ServerCert: bbsServerCert,
		ServerKey:  bbsServerKey,
		ClientCert: clientCrt,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	locketSSLConfig := world.SSLConfig{
		ServerCert: bbsServerCert,
		ServerKey:  bbsServerKey,
		ClientCert: clientCrt,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	repSSLConfig := world.SSLConfig{
		ServerCert: repServerCert,
		ServerKey:  repServerKey,
		ClientCert: clientCrt,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	auctioneerSSLConfig := world.SSLConfig{
		ServerCert: auctioneerServerCert,
		ServerKey:  auctioneerServerKey,
		ClientCert: clientCrt,
		ClientKey:  clientKey,
		CACert:     caCert,
	}

	storeTimestamp := time.Now().UnixNano

	unprivilegedGrootfsConfig := world.GrootFSConfig{
		StorePath: fmt.Sprintf("/mnt/btrfs/unprivileged-%d-%d", ginkgo.GinkgoParallelNode(), storeTimestamp),
		DraxBin:   "/usr/local/bin/drax",
		LogLevel:  "debug",
	}
	unprivilegedGrootfsConfig.Create.JSON = true
	unprivilegedGrootfsConfig.Create.UidMappings = []string{"0:4294967294:1", "1:1:4294967293"}
	unprivilegedGrootfsConfig.Create.GidMappings = []string{"0:4294967294:1", "1:1:4294967293"}

	privilegedGrootfsConfig := world.GrootFSConfig{
		StorePath: fmt.Sprintf("/mnt/btrfs/privileged-%d-%d", ginkgo.GinkgoParallelNode(), storeTimestamp),
		DraxBin:   "/usr/local/bin/drax",
		LogLevel:  "debug",
	}
	privilegedGrootfsConfig.Create.JSON = true

	gardenConfig := world.GardenSettingsConfig{
		GrootFSBinPath:            grootfsBinPath,
		GardenBinPath:             gardenBinPath,
		GardenGraphPath:           gardenGraphPath,
		UnprivilegedGrootfsConfig: unprivilegedGrootfsConfig,
		PrivilegedGrootfsConfig:   privilegedGrootfsConfig,
	}

	guid, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	volmanConfigDir, err := ioutil.TempDir(os.TempDir(), guid.String())
	Expect(err).NotTo(HaveOccurred())

	return world.ComponentMaker{
		Artifacts: builtArtifacts,
		Addresses: addresses,

		PreloadedStackPathMap: stackPathMap,

		ExternalAddress: externalAddress,

		GardenConfig:          gardenConfig,
		SSHConfig:             sshKeys,
		BbsSSL:                sslConfig,
		LocketSSL:             locketSSLConfig,
		RepSSL:                repSSLConfig,
		AuctioneerSSL:         auctioneerSSLConfig,
		SQLCACertFile:         sqlCACert,
		VolmanDriverConfigDir: volmanConfigDir,

		DBDriverName:           dbDriverName,
		DBBaseConnectionString: dbBaseConnectionString,
	}
}
