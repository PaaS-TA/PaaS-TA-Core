package main_test

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"testing"
	"time"
)

var (
	etcdPort    int
	etcdUrl     string
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
	etcdAdapter storeadapter.StoreAdapter

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

	sqlDBName string
	sqlDB     *sql.DB
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
		routingAPIBinPath = string(routingAPIBin)
		SetDefaultEventuallyTimeout(15 * time.Second)
		createSqlDatabase()
	},
)

var _ = SynchronizedAfterSuite(func() {
	dropSqlDatabase()
},
	func() {
		gexec.CleanupBuildArtifacts()
	})

func createSqlDatabase() {
	var err error
	sqlDBName = fmt.Sprintf("test%d", GinkgoParallelNode())
	sqlDB, err = sql.Open("mysql", "root:password@/")
	Expect(err).NotTo(HaveOccurred())
	Expect(sqlDB).NotTo(BeNil())
	Expect(sqlDB.Ping()).NotTo(HaveOccurred())

	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", sqlDBName))
	Expect(err).NotTo(HaveOccurred())
}

func dropSqlDatabase() {
	defer sqlDB.Close()
	_, err := sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
	Expect(err).NotTo(HaveOccurred())
}

func setupETCD() {
	etcdPort = 4001 + GinkgoParallelNode()
	etcdUrl = fmt.Sprintf("http://127.0.0.1:%d", etcdPort)
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()

	etcdVersionUrl := etcdRunner.NodeURLS()[0] + "/version"
	resp, err := http.Get(etcdVersionUrl)
	Expect(err).ToNot(HaveOccurred())

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	// response body: {"etcdserver":"2.1.1","etcdcluster":"2.1.0"}
	Expect(string(body)).To(ContainSubstring(etcdVersion))

	etcdAdapter = etcdRunner.Adapter(nil)

}

func teardownETCD() {
	etcdAdapter.Disconnect()
	etcdRunner.Reset()
	etcdRunner.Stop()
	etcdRunner.KillWithFire()
}

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

var _ = BeforeEach(func() {
	client = routingApiClient()
	setupETCD()
	setupOauthServer()

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

var _ = AfterEach(func() {
	oauthServer.Close()
	teardownETCD()
})

func createConfig(useSQL bool) string {
	type customConfig struct {
		EtcdPort int
		Port     int
		UAAPort  string
		CACerts  string
		Schema   string
	}
	caCertsPath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "uaa-certs", "uaa-ca.pem"))
	Expect(err).NotTo(HaveOccurred())

	actualConfig := customConfig{
		Port:     8125 + GinkgoParallelNode(),
		UAAPort:  oauthServerPort,
		CACerts:  caCertsPath,
		EtcdPort: etcdPort,
		Schema:   sqlDBName,
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
	configFile.Close()
	Expect(err).NotTo(HaveOccurred())

	return configFilePath
}

func getServerPort(url string) string {
	endpoints := strings.Split(url, ":")
	Expect(endpoints).To(HaveLen(3))
	return endpoints[2]
}
