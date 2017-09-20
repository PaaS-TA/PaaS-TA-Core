package benchmarkbbs_test

import (
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/db"
	etcddb "code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/sqldb"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/guidprovider"
	"code.cloudfoundry.org/benchmarkbbs/generator"
	"code.cloudfoundry.org/benchmarkbbs/reporter"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/coreos/go-etcd/etcd"
	"github.com/zorkian/go-datadog-api"

	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	bbsAddress    string
	bbsClientCert string
	bbsClientKey  string

	etcdFlags *ETCDFlags

	dataDogAPIKey string
	dataDogAppKey string

	awsAccessKeyID     string
	awsSecretAccessKey string
	awsBucketName      string
	awsRegion          string

	desiredLRPs     int
	encryptionFlags *encryption.EncryptionFlags
	metricPrefix    string

	numTrials          int
	numReps            int
	numPopulateWorkers int
	percentWrites      float64

	expectedLRPCount     int
	expectedLRPVariation float64

	expectedActualLRPCounts     map[string]int
	expectedActualLRPVariations map[string]float64

	errorTolerance float64

	logLevel    string
	logFilename string

	databaseConnectionString string
	databaseDriver           string

	logger               lager.Logger
	etcdClient           *etcd.Client
	etcdDB               *etcddb.ETCDDB
	sqlDB                *sqldb.SQLDB
	activeDB             db.DB
	bbsClient            bbs.InternalClient
	bbsClientHTTPTimeout time.Duration
	dataDogClient        *datadog.Client
	dataDogReporter      reporter.DataDogReporter
	reporters            []Reporter
)

const (
	DEBUG = "debug"
	INFO  = "info"
	ERROR = "error"
	FATAL = "fatal"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.IntVar(&numTrials, "numTrials", 10, "number of benchmark trials to average across")
	flag.IntVar(&numReps, "numReps", 10, "number of reps to simulate")
	flag.IntVar(&numPopulateWorkers, "numPopulateWorkers", 10, "number of workers generating desired LRPs during setup")
	flag.IntVar(&desiredLRPs, "desiredLRPs", 0, "number of single instance DesiredLRPs to create")
	flag.Float64Var(&percentWrites, "percentWrites", 5.0, "percentage of actual LRPs to write on each rep bulk loop")

	flag.StringVar(&databaseConnectionString, "databaseConnectionString", "", "Connection string for a MySQL database")
	flag.StringVar(&databaseDriver, "databaseDriver", "mysql", "SQL database driver name")

	flag.StringVar(&bbsAddress, "bbsAddress", "", "Address of the BBS Server")
	flag.StringVar(&bbsClientCert, "bbsClientCert", "", "BBS client SSL certificate")
	flag.StringVar(&bbsClientKey, "bbsClientKey", "", "BBS client SSL key")
	flag.DurationVar(&bbsClientHTTPTimeout, "bbsClientHTTPTimeout", 0, "BBS client HTTP timeout")

	flag.StringVar(&dataDogAPIKey, "dataDogAPIKey", "", "DataDog API key")
	flag.StringVar(&dataDogAppKey, "dataDogAppKey", "", "DataDog app Key")
	flag.StringVar(&metricPrefix, "metricPrefix", "", "DataDog metric prefix")

	flag.StringVar(&awsAccessKeyID, "awsAccessKeyID", "", "AWS Access Key ID")
	flag.StringVar(&awsSecretAccessKey, "awsSecretAccessKey", "", "AWS Secret Access Key")
	flag.StringVar(&awsBucketName, "awsBucketName", "", "AWS Bucket to store metrics")
	flag.StringVar(&awsRegion, "awsRegion", "us-west-1", "AWS Bucket to store metrics")

	flag.StringVar(&logLevel, "logLevel", string(INFO), "log level: debug, info, error or fatal")
	flag.StringVar(&logFilename, "logFilename", "", "Name of local file to save logs to")
	flag.Float64Var(&errorTolerance, "errorTolerance", 0.05, "error tollerance rate")

	etcdFlags = AddETCDFlags(flag.CommandLine)
	encryptionFlags = encryption.AddEncryptionFlags(flag.CommandLine)

	flag.Parse()

	if bbsAddress == "" {
		log.Fatal("bbsAddress is required")
	}

	BenchmarkTests(numReps, numTrials)
}

func TestBenchmarkBbs(t *testing.T) {
	var lagerLogLevel lager.LogLevel
	switch logLevel {
	case DEBUG:
		lagerLogLevel = lager.DEBUG
	case INFO:
		lagerLogLevel = lager.INFO
	case ERROR:
		lagerLogLevel = lager.ERROR
	case FATAL:
		lagerLogLevel = lager.FATAL
	default:
		panic(fmt.Errorf("unknown log level: %s", logLevel))
	}

	var logWriter io.Writer
	if logFilename == "" {
		logWriter = GinkgoWriter
	} else {
		logFile, err := os.Create(logFilename)
		if err != nil {
			panic(fmt.Errorf("Error opening file '%s': %s", logFilename, err.Error()))
		}
		defer logFile.Close()

		logWriter = logFile
	}

	logger = lager.NewLogger("bbs-benchmarks-test")
	logger.RegisterSink(lager.NewWriterSink(logWriter, lagerLogLevel))

	reporters = []Reporter{}

	if dataDogAPIKey != "" && dataDogAppKey != "" {
		dataDogClient = datadog.NewClient(dataDogAPIKey, dataDogAppKey)
		dataDogReporter = reporter.NewDataDogReporter(logger, metricPrefix, dataDogClient)
		reporters = append(reporters, &dataDogReporter)
	}

	if awsAccessKeyID != "" && awsSecretAccessKey != "" && awsBucketName != "" {
		creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")
		s3Client := s3.New(&aws.Config{
			Region:      &awsRegion,
			Credentials: creds,
		})
		uploader := s3manager.NewUploader(&s3manager.UploadOptions{S3: s3Client})
		reporter := reporter.NewS3Reporter(logger, awsBucketName, uploader)
		reporters = append(reporters, &reporter)
	}

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Benchmark BBS Suite", reporters)
}

type expectedLRPCounts struct {
	DesiredLRPCount     int
	DesiredLRPVariation float64

	ActualLRPCounts     map[string]int
	ActualLRPVariations map[string]float64
}

func initializeActiveDB() *sql.DB {
	if activeDB != nil {
		return nil
	}

	if databaseConnectionString == "" {
		etcdOptions, err := etcdFlags.Validate()
		Expect(err).NotTo(HaveOccurred())

		etcdClient = initializeEtcdClient(logger, etcdOptions)
		etcdDB = initializeETCDDB(logger, etcdClient)

		activeDB = etcdDB
		return nil
	}

	if databaseDriver == "postgres" && !strings.Contains(databaseConnectionString, "sslmode") {
		databaseConnectionString = fmt.Sprintf("%s?sslmode=disable", databaseConnectionString)
	}

	sqlConn, err := sql.Open(databaseDriver, databaseConnectionString)
	if err != nil {
		logger.Fatal("failed-to-open-sql", err)
	}
	sqlConn.SetMaxOpenConns(1)
	sqlConn.SetMaxIdleConns(1)

	err = sqlConn.Ping()
	Expect(err).NotTo(HaveOccurred())

	sqlDB = initializeSQLDB(logger, sqlConn)
	activeDB = sqlDB
	return sqlConn
}

var _ = BeforeSuite(func() {
	bbsClient = initializeBBSClient(logger, bbsClientHTTPTimeout)

	if conn := initializeActiveDB(); conn != nil {
		cleanupSQLDB(conn)
	} else {
		cleanupETCD()
	}

	_, err := bbsClient.Domains(logger)
	Expect(err).NotTo(HaveOccurred())

	expectedActualLRPVariations = make(map[string]float64)

	if desiredLRPs > 0 {
		desiredLRPGenerator := generator.NewDesiredLRPGenerator(errorTolerance, metricPrefix, numPopulateWorkers, bbsClient, dataDogClient)
		expectedLRPCount, expectedActualLRPCounts, err = desiredLRPGenerator.Generate(logger, numReps, desiredLRPs)
		Expect(err).NotTo(HaveOccurred())
		expectedLRPVariation = float64(expectedLRPCount) * errorTolerance

		for k, v := range expectedActualLRPCounts {
			expectedActualLRPVariations[k] = float64(v) * errorTolerance
		}
	}
})

var _ = AfterSuite(func() {
	if databaseConnectionString == "" {
		cleanupETCD()
	} else {
		sqlConn, err := sql.Open(databaseDriver, databaseConnectionString)
		if err != nil {
			logger.Fatal("failed-to-open-sql", err)
		}
		sqlConn.SetMaxOpenConns(1)
		sqlConn.SetMaxIdleConns(1)

		err = sqlConn.Ping()
		Expect(err).NotTo(HaveOccurred())
		cleanupSQLDB(sqlConn)
	}
})

type ETCDFlags struct {
	etcdCertFile           string
	etcdKeyFile            string
	etcdCaFile             string
	clusterUrls            string
	clientSessionCacheSize int
	maxIdleConnsPerHost    int
}

func AddETCDFlags(flagSet *flag.FlagSet) *ETCDFlags {
	flags := &ETCDFlags{}

	flagSet.StringVar(
		&flags.clusterUrls,
		"etcdCluster",
		"http://127.0.0.1:4001",
		"comma-separated list of etcd URLs (scheme://ip:port)",
	)
	flagSet.StringVar(
		&flags.etcdCertFile,
		"etcdCertFile",
		"",
		"Location of the client certificate for mutual auth",
	)
	flagSet.StringVar(
		&flags.etcdKeyFile,
		"etcdKeyFile",
		"",
		"Location of the client key for mutual auth",
	)
	flagSet.StringVar(
		&flags.etcdCaFile,
		"etcdCaFile",
		"",
		"Location of the CA certificate for mutual auth",
	)

	flagSet.IntVar(
		&flags.clientSessionCacheSize,
		"etcdSessionCacheSize",
		0,
		"Capacity of the ClientSessionCache option on the TLS configuration. If zero, golang's default will be used",
	)
	flagSet.IntVar(
		&flags.maxIdleConnsPerHost,
		"etcdMaxIdleConnsPerHost",
		0,
		"Controls the maximum number of idle (keep-alive) connctions per host. If zero, golang's default will be used",
	)
	return flags
}

func (flags *ETCDFlags) Validate() (*etcddb.ETCDOptions, error) {
	scheme := ""
	clusterUrls := strings.Split(flags.clusterUrls, ",")
	for i, uString := range clusterUrls {
		uString = strings.TrimSpace(uString)
		clusterUrls[i] = uString
		u, err := url.Parse(uString)
		if err != nil {
			return nil, fmt.Errorf("Invalid cluster URL: '%s', error: [%s]", uString, err.Error())
		}
		if scheme == "" {
			if u.Scheme != "http" && u.Scheme != "https" {
				return nil, errors.New("Invalid scheme: " + uString)
			}
			scheme = u.Scheme
		} else if scheme != u.Scheme {
			return nil, fmt.Errorf("Multiple url schemes provided: %s", flags.clusterUrls)
		}
	}

	isSSL := false
	if scheme == "https" {
		isSSL = true
		if flags.etcdCertFile == "" {
			return nil, errors.New("Cert file must be provided for https connections")
		}
		if flags.etcdKeyFile == "" {
			return nil, errors.New("Key file must be provided for https connections")
		}
	}

	return &etcddb.ETCDOptions{
		CertFile:    flags.etcdCertFile,
		KeyFile:     flags.etcdKeyFile,
		CAFile:      flags.etcdCaFile,
		ClusterUrls: clusterUrls,
		IsSSL:       isSSL,
		ClientSessionCacheSize: flags.clientSessionCacheSize,
		MaxIdleConnsPerHost:    flags.maxIdleConnsPerHost,
	}, nil
}

func initializeEtcdClient(logger lager.Logger, etcdOptions *etcddb.ETCDOptions) *etcd.Client {
	var etcdClient *etcd.Client
	var tr *http.Transport

	if etcdOptions.IsSSL {
		if etcdOptions.CertFile == "" || etcdOptions.KeyFile == "" {
			logger.Fatal("failed-to-construct-etcd-tls-client", errors.New("Require both cert and key path"))
		}

		var err error
		etcdClient, err = etcd.NewTLSClient(etcdOptions.ClusterUrls, etcdOptions.CertFile, etcdOptions.KeyFile, etcdOptions.CAFile)
		if err != nil {
			logger.Fatal("failed-to-construct-etcd-tls-client", err)
		}

		tlsCert, err := tls.LoadX509KeyPair(etcdOptions.CertFile, etcdOptions.KeyFile)
		if err != nil {
			logger.Fatal("failed-to-construct-etcd-tls-client", err)
		}

		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{tlsCert},
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(etcdOptions.ClientSessionCacheSize),
		}
		tr = &http.Transport{
			TLSClientConfig:     tlsConfig,
			Dial:                etcdClient.DefaultDial,
			MaxIdleConnsPerHost: etcdOptions.MaxIdleConnsPerHost,
		}
		etcdClient.SetTransport(tr)
	} else {
		etcdClient = etcd.NewClient(etcdOptions.ClusterUrls)
	}
	etcdClient.SetConsistency(etcd.STRONG_CONSISTENCY)

	return etcdClient
}

func initializeETCDDB(logger lager.Logger, etcdClient *etcd.Client) *etcddb.ETCDDB {
	key, keys, err := encryptionFlags.Parse()
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	keyManager, err := encryption.NewKeyManager(key, keys)
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	cryptor := encryption.NewCryptor(keyManager, rand.Reader)

	return etcddb.NewETCD(format.ENCRYPTED_PROTO, 1000, 1000, 1*time.Minute, cryptor, etcddb.NewStoreClient(etcdClient), clock.NewClock())
}

func initializeSQLDB(logger lager.Logger, sqlConn *sql.DB) *sqldb.SQLDB {
	key, keys, err := encryptionFlags.Parse()
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	keyManager, err := encryption.NewKeyManager(key, keys)
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	cryptor := encryption.NewCryptor(keyManager, rand.Reader)

	return sqldb.NewSQLDB(sqlConn, 1000, 1000, format.ENCODED_PROTO, cryptor, guidprovider.DefaultGuidProvider, clock.NewClock(), databaseDriver)
}

func initializeBBSClient(logger lager.Logger, bbsClientHTTPTimeout time.Duration) bbs.InternalClient {
	bbsURL, err := url.Parse(bbsAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(bbsAddress)
	}

	cfhttp.Initialize(bbsClientHTTPTimeout)
	bbsClient, err := bbs.NewSecureSkipVerifyClient(bbsAddress, bbsClientCert, bbsClientKey, 1, 25000)
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}

func purge(key string) {
	_, err := etcdClient.Delete(key, true)
	if err != nil {
		matches, matchErr := regexp.Match(".*Key not found.*", []byte(err.Error()))
		if matchErr != nil {
			Fail(matchErr.Error())
		}
		if !matches {
			Fail(err.Error())
		}
	}
}

func cleanupETCD() {
	purge("/v1/desired_lrp")
	purge("/v1/actual")
}

func cleanupSQLDB(conn *sql.DB) {
	_, err := conn.Exec("TRUNCATE actual_lrps")
	Expect(err).NotTo(HaveOccurred())
	_, err = conn.Exec("TRUNCATE desired_lrps")
	Expect(err).NotTo(HaveOccurred())
}
