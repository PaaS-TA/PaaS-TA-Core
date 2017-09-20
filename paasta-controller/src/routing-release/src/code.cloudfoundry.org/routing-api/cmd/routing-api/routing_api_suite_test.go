package main_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"testing"
	"time"
)

var (
	etcdPort int
	etcdUrl  string

	client                 routing_api.Client
	routingAPIBinPath      string
	routingAPIAddress      string
	routingAPIArgs         testrunner.Args
	routingAPIArgsNoSQL    testrunner.Args
	routingAPIPort         uint16
	routingAPIIP           string
	routingAPISystemDomain string
	oauthServer            *ghttp.Server
	oauthServerPort        string

	sqlDBName    string
	gormDB       *gorm.DB
	consulRunner *consulrunner.ClusterRunner

	mysqlAllocator testrunner.DbAllocator
	etcdAllocator  testrunner.DbAllocator
)

var etcdVersion = "etcdserver\":\"2.1.1"

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		routingAPIBin, err := gexec.Build("code.cloudfoundry.org/routing-api/cmd/routing-api", "-race")
		Expect(err).NotTo(HaveOccurred())
		return []byte(routingAPIBin)
	},
	func(routingAPIBin []byte) {
		var err error
		routingAPIBinPath = string(routingAPIBin)
		SetDefaultEventuallyTimeout(15 * time.Second)
		etcdPort = 4001 + GinkgoParallelNode()

		mysqlAllocator = testrunner.NewMySQLAllocator()
		etcdAllocator = testrunner.NewEtcdAllocator(etcdPort)

		sqlDBName, err = mysqlAllocator.Create()
		Expect(err).NotTo(HaveOccurred())

		etcdUrl, err = etcdAllocator.Create()
		Expect(err).NotTo(HaveOccurred())

		setupConsul()
		setupOauthServer()
	},
)

var _ = SynchronizedAfterSuite(func() {
	err := mysqlAllocator.Delete()
	Expect(err).NotTo(HaveOccurred())
	err = etcdAllocator.Delete()
	Expect(err).NotTo(HaveOccurred())

	teardownConsul()
	oauthServer.Close()
},
	func() {
		gexec.CleanupBuildArtifacts()
	})

var _ = BeforeEach(func() {
	client = routingApiClient()
	err := mysqlAllocator.Reset()
	Expect(err).NotTo(HaveOccurred())
	err = etcdAllocator.Reset()
	Expect(err).NotTo(HaveOccurred())
	resetConsul()

	routingAPIArgs = testrunner.Args{
		Port:       routingAPIPort,
		IP:         routingAPIIP,
		ConfigPath: createConfig(true),
		DevMode:    true,
	}

	routingAPIArgsNoSQL = testrunner.Args{
		Port:       routingAPIPort,
		IP:         routingAPIIP,
		ConfigPath: createConfig(false),
		DevMode:    true,
	}
})

func routingApiClient() routing_api.Client {
	routingAPIPort = uint16(6900 + GinkgoParallelNode())
	routingAPIIP = "127.0.0.1"
	routingAPISystemDomain = "example.com"
	routingAPIAddress = fmt.Sprintf("%s:%d", routingAPIIP, routingAPIPort)

	routingAPIURL := &url.URL{
		Scheme: "http",
		Host:   routingAPIAddress,
	}

	return routing_api.NewClient(routingAPIURL.String(), false)
}

func setupOauthServer() {
	oauthServer = ghttp.NewUnstartedServer()
	basePath, err := filepath.Abs(path.Join("..", "..", "fixtures", "uaa-certs"))
	Expect(err).ToNot(HaveOccurred())

	cert, err := tls.LoadX509KeyPair(filepath.Join(basePath, "server.pem"), filepath.Join(basePath, "server.key"))
	Expect(err).ToNot(HaveOccurred())
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	oauthServer.HTTPTestServer.TLS = tlsConfig
	oauthServer.AllowUnhandledRequests = true
	oauthServer.UnhandledRequestStatusCode = http.StatusOK
	oauthServer.HTTPTestServer.StartTLS()

	oauthServerPort = getServerPort(oauthServer.URL())
}

func setupConsul() {
	consulRunner = consulrunner.NewClusterRunner(9001+GinkgoParallelNode()*consulrunner.PortOffsetLength, 1, "http")
	consulRunner.Start()
	consulRunner.WaitUntilReady()
}

func teardownConsul() {
	consulRunner.Stop()
}

func resetConsul() {
	_ = consulRunner.Reset()
	// TODO: https://www.pivotaltracker.com/story/show/133202225
	//	Expect(err).ToNot(HaveOccurred())
}

func createConfig(useSQL bool) string {
	type customConfig struct {
		EtcdPort  int
		Port      int
		UAAPort   string
		CACerts   string
		Schema    string
		ConsulUrl string
	}
	caCertsPath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "uaa-certs", "uaa-ca.pem"))
	Expect(err).NotTo(HaveOccurred())

	actualConfig := customConfig{
		Port:      8125 + GinkgoParallelNode(),
		UAAPort:   oauthServerPort,
		CACerts:   caCertsPath,
		EtcdPort:  etcdPort,
		Schema:    sqlDBName,
		ConsulUrl: consulRunner.URL(),
	}

	var templatePath string
	if useSQL {
		templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template_sql.yml"))
	} else {
		templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template.yml"))
	}
	Expect(err).NotTo(HaveOccurred())

	tmpl, err := template.ParseFiles(templatePath)
	Expect(err).NotTo(HaveOccurred())

	var configFilePath string
	if useSQL {
		configFilePath = fmt.Sprintf("/tmp/example_sql_%d.yml", GinkgoParallelNode())
	} else {
		configFilePath = fmt.Sprintf("/tmp/example_%d.yml", GinkgoParallelNode())
	}
	configFile, err := os.Create(configFilePath)
	Expect(err).NotTo(HaveOccurred())

	err = tmpl.Execute(configFile, actualConfig)
	defer func() {
		err := configFile.Close()
		Expect(err).ToNot(HaveOccurred())
	}()
	Expect(err).NotTo(HaveOccurred())

	return configFilePath
}

func getServerPort(url string) string {
	endpoints := strings.Split(url, ":")
	Expect(endpoints).To(HaveLen(3))
	return endpoints[2]
}
