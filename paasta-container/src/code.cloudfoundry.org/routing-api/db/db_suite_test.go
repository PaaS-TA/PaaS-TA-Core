package db_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"

	"testing"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/routing-api/config"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var etcdClient storeadapter.StoreAdapter
var etcdPort int
var etcdUrl string
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdVersion = "etcdserver\":\"2.1.1"
var routingAPIBinPath string
var basePath string
var sqlCfg *config.SqlDB
var sqlDBName string
var postgresDBName string
var mysqlAllocator testrunner.DbAllocator
var postgresAllocator testrunner.DbAllocator

func TestDB(t *testing.T) {
	var err error
	RegisterFailHandler(Fail)

	basePath, err = filepath.Abs(path.Join("..", "fixtures", "etcd-certs"))
	Expect(err).NotTo(HaveOccurred())

	serverSSLConfig := &etcdstorerunner.SSLConfig{
		CertFile: filepath.Join(basePath, "server.crt"),
		KeyFile:  filepath.Join(basePath, "server.key"),
		CAFile:   filepath.Join(basePath, "etcd-ca.crt"),
	}

	etcdPort = 4001 + GinkgoParallelNode()
	etcdUrl = fmt.Sprintf("https://127.0.0.1:%d", etcdPort)
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, serverSSLConfig)
	etcdRunner.Start()

	clientSSLConfig := &etcdstorerunner.SSLConfig{
		CertFile: filepath.Join(basePath, "client.crt"),
		KeyFile:  filepath.Join(basePath, "client.key"),
		CAFile:   filepath.Join(basePath, "etcd-ca.crt"),
	}
	etcdClient = etcdRunner.Adapter(clientSSLConfig)

	RunSpecs(t, "DB Suite")
}

var _ = BeforeSuite(func() {
	var err error

	postgresAllocator = testrunner.NewPostgresAllocator()
	postgresDBName, err = postgresAllocator.Create()
	Expect(err).ToNot(HaveOccurred())

	mysqlAllocator = testrunner.NewMySQLAllocator()
	sqlDBName, err = mysqlAllocator.Create()
	Expect(err).ToNot(HaveOccurred())
	Expect(len(etcdRunner.NodeURLS())).Should(BeNumerically(">=", 1))

	tlsConfig, err := cfhttp.NewTLSConfig(
		filepath.Join(basePath, "client.crt"),
		filepath.Join(basePath, "client.key"),
		filepath.Join(basePath, "etcd-ca.crt"))
	Expect(err).ToNot(HaveOccurred())

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: tr}

	etcdVersionUrl := etcdRunner.NodeURLS()[0] + "/version"
	resp, err := client.Get(etcdVersionUrl)
	Expect(err).ToNot(HaveOccurred())

	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	// response body: {"etcdserver":"2.1.1","etcdcluster":"2.1.0"}
	Expect(string(body)).To(ContainSubstring(etcdVersion))
})

var _ = AfterSuite(func() {

	err := mysqlAllocator.Delete()
	Expect(err).ToNot(HaveOccurred())

	err = postgresAllocator.Delete()
	Expect(err).ToNot(HaveOccurred())

	etcdRunner.KillWithFire()
})

var _ = BeforeEach(func() {
	err := mysqlAllocator.Reset()
	Expect(err).ToNot(HaveOccurred())
	err = postgresAllocator.Reset()
	Expect(err).ToNot(HaveOccurred())
	etcdRunner.Reset()
})
