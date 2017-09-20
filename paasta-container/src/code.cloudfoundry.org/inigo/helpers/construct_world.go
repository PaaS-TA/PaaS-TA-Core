package helpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/inigo/world"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/gomega"
)

var PreloadedStacks = []string{"red-stack", "blue-stack"}
var DefaultStack = PreloadedStacks[0]

var addresses world.ComponentAddresses

const (
	assetsPath = "../fixtures/certs/"
)

func MakeComponentMaker(builtArtifacts world.BuiltArtifacts, localIP string) world.ComponentMaker {
	gardenBinPath := os.Getenv("GARDEN_BINPATH")
	gardenRootFSPath := os.Getenv("GARDEN_ROOTFS")
	gardenGraphPath := os.Getenv("GARDEN_GRAPH_PATH")
	externalAddress := os.Getenv("EXTERNAL_ADDRESS")

	dbDriverName := os.Getenv("USE_SQL")
	useSQL := dbDriverName != ""

	var dbBaseConnectionString string
	if useSQL {
		if dbDriverName == "postgres" {
			dbBaseConnectionString = "postgres://diego:diego_pw@127.0.0.1/"
		} else if dbDriverName == "mysql" {
			dbBaseConnectionString = "diego:diego_password@/"
		} else {
			panic(fmt.Sprintf("Unsupported Driver: %s", dbDriverName))
		}
	}

	if gardenGraphPath == "" {
		gardenGraphPath = os.TempDir()
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
		Etcd:                fmt.Sprintf("127.0.0.1:%d", 12000+config.GinkgoConfig.ParallelNode),
		EtcdPeer:            fmt.Sprintf("127.0.0.1:%d", 12500+config.GinkgoConfig.ParallelNode),
		Consul:              fmt.Sprintf("127.0.0.1:%d", 12750+config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength),
		Rep:                 fmt.Sprintf("0.0.0.0:%d", 14000+config.GinkgoConfig.ParallelNode),
		FileServer:          fmt.Sprintf("%s:%d", localIP, 17000+config.GinkgoConfig.ParallelNode),
		Router:              fmt.Sprintf("127.0.0.1:%d", 18000+config.GinkgoConfig.ParallelNode),
		BBS:                 fmt.Sprintf("127.0.0.1:%d", 20500+config.GinkgoConfig.ParallelNode*2),
		Health:              fmt.Sprintf("127.0.0.1:%d", 20500+config.GinkgoConfig.ParallelNode*2+1),
		Auctioneer:          fmt.Sprintf("0.0.0.0:%d", 23000+config.GinkgoConfig.ParallelNode),
		SSHProxy:            fmt.Sprintf("127.0.0.1:%d", 23500+config.GinkgoConfig.ParallelNode),
		SSHProxyHealthCheck: fmt.Sprintf("127.0.0.1:%d", 24500+config.GinkgoConfig.ParallelNode),
		FakeVolmanDriver:    fmt.Sprintf("127.0.0.1:%d", 25500+config.GinkgoConfig.ParallelNode),
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
	clientCrt, err := filepath.Abs(assetsPath + "client.crt")
	Expect(err).NotTo(HaveOccurred())
	clientKey, err := filepath.Abs(assetsPath + "client.key")
	Expect(err).NotTo(HaveOccurred())
	caCert, err := filepath.Abs(assetsPath + "ca.crt")
	Expect(err).NotTo(HaveOccurred())

	sslConfig := world.SSLConfig{
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

	guid, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())

	volmanConfigDir, err := ioutil.TempDir(os.TempDir(), guid.String())
	Expect(err).NotTo(HaveOccurred())

	return world.ComponentMaker{
		Artifacts: builtArtifacts,
		Addresses: addresses,

		PreloadedStackPathMap: stackPathMap,

		ExternalAddress: externalAddress,

		GardenBinPath:         gardenBinPath,
		GardenGraphPath:       gardenGraphPath,
		SSHConfig:             sshKeys,
		EtcdSSL:               sslConfig,
		BbsSSL:                sslConfig,
		RepSSL:                repSSLConfig,
		VolmanDriverConfigDir: volmanConfigDir,

		DBDriverName:           dbDriverName,
		DBBaseConnectionString: dbBaseConnectionString,
		UseSQL:                 useSQL,
	}
}
