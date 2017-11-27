package main_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"google.golang.org/grpc/grpclog"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/gorouter/config"
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
	locketBinPath          string
	routingAPIBinPath      string
	routingAPIAddress      string
	routingAPIConfig       config.Config
	routingAPIArgs         testrunner.Args
	routingAPIArgsNoSQL    testrunner.Args
	routingAPIArgsOnlySQL  testrunner.Args
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

		locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
		Expect(err).NotTo(HaveOccurred())

		return []byte(strings.Join([]string{routingAPIBin, locketPath}, ","))
	},
	func(binPaths []byte) {
		grpclog.SetLogger(log.New(ioutil.Discard, "", 0))

		var err error
		path := string(binPaths)
		routingAPIBinPath = strings.Split(path, ",")[0]
		locketBinPath = strings.Split(path, ",")[1]

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
}, func() {
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
		ConfigPath: createConfig(true, true),
		DevMode:    true,
	}

	routingAPIArgsNoSQL = testrunner.Args{
		Port:       routingAPIPort,
		IP:         routingAPIIP,
		ConfigPath: createConfig(false, true),
		DevMode:    true,
	}

	routingAPIArgsOnlySQL = testrunner.Args{
		Port:       routingAPIPort,
		IP:         routingAPIIP,
		ConfigPath: createConfig(true, false),
		DevMode:    true,
	}
})

func routingApiClient() routing_api.Client {
	routingAPIPort = uint16(testPort())
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
	consulRunner = consulrunner.NewClusterRunner(consulrunner.ClusterRunnerConfig{
		StartingPort: 9001 + GinkgoParallelNode()*consulrunner.PortOffsetLength,
		NumNodes:     1,
		Scheme:       "http",
	})
	consulRunner.Start()
	consulRunner.WaitUntilReady()
}

func teardownConsul() {
	consulRunner.Stop()
}

func resetConsul() {
	err := consulRunner.Reset()
	Expect(err).ToNot(HaveOccurred())
}

func createConfigWithRg() string {
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
	templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template_rg.yml"))
	Expect(err).NotTo(HaveOccurred())

	var configFilePath string
	configFilePath = fmt.Sprintf("/tmp/example_rg_%d.yml", GinkgoParallelNode())

	return writeConfigFile(templatePath, configFilePath, actualConfig)
}

type customConfig struct {
	EtcdPort  int
	Port      int
	UAAPort   string
	CACerts   string
	Schema    string
	ConsulUrl string
}

func createConfig(useSQL bool, useETCD bool) string {
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
	if useSQL && useETCD {
		templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template_sql.yml"))
	} else if useSQL {
		templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template_sql_only.yml"))
	} else if useETCD {
		templatePath, err = filepath.Abs(filepath.Join("..", "..", "example_config", "example_template.yml"))
	} else {
		err = errors.New("Invalid database selection")
	}
	Expect(err).NotTo(HaveOccurred())

	var configFilePath string
	if useSQL && useETCD {
		configFilePath = fmt.Sprintf("/tmp/example_sql_%d.yml", GinkgoParallelNode())
	} else if useSQL {
		configFilePath = fmt.Sprintf("/tmp/example_sql_only_%d.yml", GinkgoParallelNode())
	} else {
		configFilePath = fmt.Sprintf("/tmp/example_%d.yml", GinkgoParallelNode())
	}
	return writeConfigFile(templatePath, configFilePath, actualConfig)
}

func writeConfigFile(templatePath, configFilePath string, actualConfig customConfig) string {
	tmpl, err := template.ParseFiles(templatePath)
	Expect(err).NotTo(HaveOccurred())

	configFile, err := os.Create(configFilePath)
	Expect(err).NotTo(HaveOccurred())

	err = tmpl.Execute(configFile, actualConfig)
	defer func() {
		closeErr := configFile.Close()
		Expect(closeErr).ToNot(HaveOccurred())
	}()
	Expect(err).NotTo(HaveOccurred())

	return configFilePath
}
func getServerPort(url string) string {
	endpoints := strings.Split(url, ":")
	Expect(endpoints).To(HaveLen(3))
	return endpoints[2]
}

func testPort() int {
	add, err := net.ResolveTCPAddr("tcp", ":0")
	Expect(err).NotTo(HaveOccurred())
	l, err := net.ListenTCP("tcp", add)
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		err = l.Close()
		Expect(err).NotTo(HaveOccurred())
	}()

	return l.Addr().(*net.TCPAddr).Port
}

func validatePort(port uint16) {
	Eventually(func() error {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if l != nil {
			_ = l.Close()
		}
		return err
	}, "60s", "1s").Should(BeNil())
}
