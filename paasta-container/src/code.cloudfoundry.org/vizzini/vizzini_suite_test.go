package vizzini_test

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/say"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
)

var bbsClient bbs.InternalClient
var domain string
var otherDomain string
var defaultRootFS string
var guid string
var startTime time.Time

var bbsAddress string
var bbsClientCert string
var bbsClientKey string
var routableDomainSuffix string
var sshAddress string
var sshHost string
var sshPort string
var sshPassword string
var hostAddress string
var logger lager.Logger

var timeout time.Duration
var dockerTimeout time.Duration

func init() {
	flag.StringVar(&bbsAddress, "bbs-address", "http://10.244.16.2:8889", "http address for the bbs (required)")
	flag.StringVar(&bbsClientCert, "bbs-client-cert", "", "bbs client ssl certificate")
	flag.StringVar(&bbsClientKey, "bbs-client-key", "", "bbs client ssl key")
	flag.StringVar(&sshAddress, "ssh-address", "ssh.bosh-lite.com:2222", "domain and port for the ssh proxy (required)")
	flag.StringVar(&sshPassword, "ssh-password", "bosh-lite-ssh-secret", "password for the ssh proxy's diego authenticator")
	flag.StringVar(&routableDomainSuffix, "routable-domain-suffix", "bosh-lite.com", "suffix to use when constructing FQDN")
	flag.StringVar(&hostAddress, "host-address", "10.0.2.2", "address that a process running in a container on Diego can use to reach the machine running this test.  Typically the gateway on the vagrant VM.")
	flag.Parse()

	if bbsAddress == "" {
		log.Fatal("i need a bbs address to talk to Diego...")
	}

	if sshAddress == "" {
		log.Fatal("i need an SSH address to talk to Diego...")
	}
}

func TestVizziniSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vizzini Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())
	return domain + "-" + u.String()[:8]
}

var _ = BeforeSuite(func() {
	var err error
	timeout = 10 * time.Second
	dockerTimeout = 120 * time.Second

	timeoutArg := os.Getenv("DEFAULT_EVENTUALLY_TIMEOUT")
	if timeoutArg != "" {
		timeout, err = time.ParseDuration(timeoutArg)
		Expect(err).NotTo(HaveOccurred(), "invalid value '"+timeoutArg+"' for DEFAULT_EVENTUALLY_TIMEOUT")
		fmt.Printf("Setting Default Eventually Timeout to %s\n", timeout)
	}

	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)
	SetDefaultConsistentlyPollingInterval(200 * time.Millisecond)
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelNode())
	otherDomain = fmt.Sprintf("vizzini-other-%d", GinkgoParallelNode())
	defaultRootFS = models.PreloadedRootFS("cflinuxfs2")

	bbsClient = initializeBBSClient()

	sshHost, sshPort, err = net.SplitHostPort(sshAddress)
	Expect(err).NotTo(HaveOccurred())

	logger = lagertest.NewTestLogger("vizzini")
})

var _ = BeforeEach(func() {
	startTime = time.Now()
	guid = NewGuid()
})

var _ = AfterEach(func() {
	defer func() {
		endTime := time.Now()
		fmt.Fprint(GinkgoWriter, say.Cyan("\n%s\nThis test referenced GUID %s\nStart time: %s (%d)\nEnd time: %s (%d)\n", CurrentGinkgoTestDescription().FullTestText, guid, startTime, startTime.Unix(), endTime, endTime.Unix()))
	}()

	for _, domain := range []string{domain, otherDomain} {
		ClearOutTasksInDomain(domain)
		ClearOutDesiredLRPsInDomain(domain)
	}
})

var _ = AfterSuite(func() {
	for _, domain := range []string{domain, otherDomain} {
		bbsClient.UpsertDomain(logger, domain, 5*time.Minute) //leave the domain around forever so that Diego cleans up if need be
	}

	for _, domain := range []string{domain, otherDomain} {
		ClearOutDesiredLRPsInDomain(domain)
		ClearOutTasksInDomain(domain)
	}
})

func initializeBBSClient() bbs.InternalClient {
	bbsURL, err := url.Parse(bbsAddress)
	Expect(err).NotTo(HaveOccurred())

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(bbsAddress)
	}

	bbsClient, err := bbs.NewSecureSkipVerifyClient(bbsAddress, bbsClientCert, bbsClientKey, 0, 0)
	Expect(err).NotTo(HaveOccurred())
	return bbsClient
}
