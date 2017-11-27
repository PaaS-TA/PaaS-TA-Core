package benchmarkbbs_test

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/db/sqldb"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/guidprovider"
	benchmarkconfig "code.cloudfoundry.org/benchmarkbbs/config"
	"code.cloudfoundry.org/benchmarkbbs/generator"
	"code.cloudfoundry.org/benchmarkbbs/reporter"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/clock"
	fakes "code.cloudfoundry.org/go-loggregator/testhelpers/fakes/v1"
	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/zorkian/go-datadog-api"

	_ "github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	expectedLRPCount int

	expectedActualLRPCounts map[string]int
	config                  benchmarkconfig.BenchmarkBBSConfig

	logger          lager.Logger
	sqlDB           *sqldb.SQLDB
	activeDB        db.DB
	bbsClient       bbs.InternalClient
	dataDogClient   *datadog.Client
	dataDogReporter reporter.DataDogReporter
	reporters       []Reporter
)

const (
	DEBUG = "debug"
	INFO  = "info"
	ERROR = "error"
	FATAL = "fatal"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	configFile := flag.String("config", "", "config file")
	flag.Parse()

	var err error
	config, err = benchmarkconfig.NewBenchmarkBBSConfig(*configFile)
	if err != nil {
		panic(err)
	}

	if config.BBSAddress == "" {
		log.Fatal("bbsAddress is required")
	}

	BenchmarkTests(config.NumReps, config.NumTrials, config.LocalRouteEmitters)
}

func TestBenchmarkBbs(t *testing.T) {
	var lagerLogLevel lager.LogLevel
	switch config.LogLevel {
	case DEBUG:
		lagerLogLevel = lager.DEBUG
	case INFO:
		lagerLogLevel = lager.INFO
	case ERROR:
		lagerLogLevel = lager.ERROR
	case FATAL:
		lagerLogLevel = lager.FATAL
	default:
		panic(fmt.Errorf("unknown log level: %s", config.LogLevel))
	}

	var logWriter io.Writer
	if config.LogFilename == "" {
		logWriter = GinkgoWriter
	} else {
		var logFile *os.File
		var err error
		if _, err = os.Stat(config.LogFilename); os.IsNotExist(err) {
			logFile, err = os.Create(config.LogFilename)
			if err != nil {
				panic(fmt.Errorf("Error opening file '%s': %s", config.LogFilename, err.Error()))
			}
		} else {
			logFile, err = os.OpenFile(config.LogFilename, os.O_APPEND, 0600)
		}

		defer logFile.Close()

		logWriter = logFile
	}

	logger = lager.NewLogger("bbs-benchmarks-test")
	logger.RegisterSink(lager.NewWriterSink(logWriter, lagerLogLevel))

	reporters = []Reporter{}

	if config.DataDogAPIKey != "" && config.DataDogAppKey != "" {
		dataDogClient = datadog.NewClient(config.DataDogAPIKey, config.DataDogAppKey)
		dataDogReporter = reporter.NewDataDogReporter(logger, config.MetricPrefix, dataDogClient)
		reporters = append(reporters, &dataDogReporter)
	}

	if config.AwsAccessKeyID != "" && config.AwsSecretAccessKey != "" && config.AwsBucketName != "" {
		creds := credentials.NewStaticCredentials(config.AwsAccessKeyID, config.AwsSecretAccessKey, "")
		s3Client := s3.New(&aws.Config{
			Region:      &config.AwsRegion,
			Credentials: creds,
		})
		uploader := s3manager.NewUploader(&s3manager.UploadOptions{S3: s3Client})
		reporter := reporter.NewS3Reporter(logger, config.AwsBucketName, uploader)
		reporters = append(reporters, &reporter)
	}

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Benchmark BBS Suite", reporters)
}

type expectedLRPCounts struct {
	DesiredLRPCount int

	ActualLRPCounts map[string]int
}

func initializeActiveDB() *sql.DB {
	if activeDB != nil {
		return nil
	}

	if config.DatabaseConnectionString == "" {
		logger.Fatal("no-sql-configuration", errors.New("no-sql-configuration"))
	}

	if config.DatabaseDriver == "postgres" && !strings.Contains(config.DatabaseConnectionString, "sslmode") {
		config.DatabaseConnectionString = fmt.Sprintf("%s?sslmode=disable", config.DatabaseConnectionString)
	}

	sqlConn, err := sql.Open(config.DatabaseDriver, config.DatabaseConnectionString)
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
	bbsClient = initializeBBSClient(logger, time.Duration(config.BBSClientHTTPTimeout))

	conn := initializeActiveDB()

	if config.ReseedDatabase {
		var err error

		cleanupSQLDB(conn)

		if config.DesiredLRPs > 0 {
			desiredLRPGenerator := generator.NewDesiredLRPGenerator(
				config.ErrorTolerance,
				config.MetricPrefix,
				config.NumPopulateWorkers,
				bbsClient,
				dataDogClient,
			)

			expectedLRPCount, expectedActualLRPCounts, err = desiredLRPGenerator.Generate(logger, config.NumReps, config.DesiredLRPs)
			Expect(err).NotTo(HaveOccurred())
		}
	} else {
		expectedActualLRPCounts = make(map[string]int)
		query := `
			SELECT
				COUNT(*)
			FROM desired_lrps
		`
		res := conn.QueryRow(query)
		err := res.Scan(&expectedLRPCount)
		Expect(err).NotTo(HaveOccurred())

		query = `
			SELECT
				cell_id, COUNT(*)
			FROM actual_lrps
			GROUP BY cell_id
		`
		rows, err := conn.Query(query)
		Expect(err).NotTo(HaveOccurred())

		defer rows.Close()
		for rows.Next() {
			var cellId string
			var count int
			err = rows.Scan(&cellId, &count)
			Expect(err).NotTo(HaveOccurred())

			expectedActualLRPCounts[cellId] = count
		}
	}

	if float64(expectedLRPCount) < float64(config.DesiredLRPs)*config.ErrorTolerance {
		Fail(fmt.Sprintf("Error rate of %.3f for actuals exceeds tolerance of %.3f", float64(expectedLRPCount)/float64(config.DesiredLRPs), config.ErrorTolerance))
	}
})

func initializeSQLDB(logger lager.Logger, sqlConn *sql.DB) *sqldb.SQLDB {
	key, keys, err := config.EncryptionConfig.Parse()
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	keyManager, err := encryption.NewKeyManager(key, keys)
	if err != nil {
		logger.Fatal("cannot-setup-encryption", err)
	}
	cryptor := encryption.NewCryptor(keyManager, rand.Reader)

	return sqldb.NewSQLDB(
		sqlConn,
		1000,
		1000,
		format.ENCODED_PROTO,
		cryptor,
		guidprovider.DefaultGuidProvider,
		clock.NewClock(),
		config.DatabaseDriver,
		&fakes.FakeIngressClient{},
	)
}

func initializeBBSClient(logger lager.Logger, bbsClientHTTPTimeout time.Duration) bbs.InternalClient {
	bbsURL, err := url.Parse(config.BBSAddress)
	if err != nil {
		logger.Fatal("Invalid BBS URL", err)
	}

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(config.BBSAddress)
	}

	cfhttp.Initialize(bbsClientHTTPTimeout)
	var bbsClient bbs.InternalClient
	if config.SkipCertVerify {
		bbsClient, err = bbs.NewSecureSkipVerifyClient(
			config.BBSAddress,
			config.BBSClientCert,
			config.BBSClientKey,
			1,
			25000,
		)
	} else {
		bbsClient, err = bbs.NewSecureClient(
			config.BBSAddress,
			config.BBSCACert,
			config.BBSClientCert,
			config.BBSClientKey,
			1,
			25000,
		)
	}
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}

func cleanupSQLDB(conn *sql.DB) {
	_, err := conn.Exec("TRUNCATE actual_lrps")
	Expect(err).NotTo(HaveOccurred())
	_, err = conn.Exec("TRUNCATE desired_lrps")
	Expect(err).NotTo(HaveOccurred())
}
