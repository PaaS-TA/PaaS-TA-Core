package db_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"

	"testing"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/routing-api/config"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	_ "github.com/go-sql-driver/mysql"
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
var sqlDB *sql.DB

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

	etcdRunner.Stop()
}

var _ = BeforeSuite(func() {
	sqlDBName = fmt.Sprintf("test%d", GinkgoParallelNode())
	var err error
	sqlDB, err = sql.Open("mysql", "root:password@/")
	Expect(err).NotTo(HaveOccurred())
	Expect(sqlDB).NotTo(BeNil())
	Expect(sqlDB.Ping()).NotTo(HaveOccurred())

	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", sqlDBName))
	Expect(err).NotTo(HaveOccurred())

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

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	// response body: {"etcdserver":"2.1.1","etcdcluster":"2.1.0"}
	Expect(string(body)).To(ContainSubstring(etcdVersion))
})

var _ = AfterSuite(func() {
	defer sqlDB.Close()
	_, err := sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", sqlDBName))
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
