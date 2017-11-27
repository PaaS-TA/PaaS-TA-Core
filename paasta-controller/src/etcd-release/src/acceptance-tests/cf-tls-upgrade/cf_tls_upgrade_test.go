package cf_tls_upgrade_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/logspammer"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/syslogchecker"
	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf-experimental/bosh-test/bosh"
	"github.com/pivotal-cf-experimental/destiny/ops"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	CF_PUSH_TIMEOUT                       = 2 * time.Minute
	DEFAULT_TIMEOUT                       = 30 * time.Second
	GUID_NOT_FOUND_ERROR_THRESHOLD        = 1
	GATEWAY_TIMEOUT_ERROR_COUNT_THRESHOLD = 2
	BAD_GATEWAY_ERROR_COUNT_THRESHOLD     = 2
	MISSING_LOG_THRESHOLD                 = 600 // Frequency of spammer is 100ms (allow 60s of missing logs)
)

type gen struct{}

func (gen) Generate() string {
	return strconv.Itoa(rand.Int())
}

type runner struct{}

func (runner) Run(args ...string) ([]byte, error) {
	return exec.Command("cf", args...).CombinedOutput()
}

var _ = Describe("CF TLS Upgrade Test", func() {
	It("successfully upgrades etcd cluster to use TLS", func() {
		var (
			nonTLSManifest   string
			etcdTLSManifest  string
			proxyTLSManifest string
			manifestName     string

			err     error
			appName string

			spammer *logspammer.Spammer
			checker syslogchecker.Checker
		)

		varsStoreBytes, err := ioutil.ReadFile(config.BOSH.DeploymentVarsPath)
		Expect(err).NotTo(HaveOccurred())
		varsStore := string(varsStoreBytes)

		var getToken = func() string {
			session := cf.Cf("oauth-token")
			Eventually(session, DEFAULT_TIMEOUT).Should(gexec.Exit(0))

			token := strings.TrimSpace(string(session.Out.Contents()))
			Expect(token).NotTo(Equal(""))
			return token
		}

		var getAppGuid = func(appName string) string {
			cfApp := cf.Cf("app", appName, "--guid")
			Eventually(cfApp, DEFAULT_TIMEOUT).Should(gexec.Exit(0))

			appGuid := strings.TrimSpace(string(cfApp.Out.Contents()))
			Expect(appGuid).NotTo(Equal(""))
			return appGuid
		}

		var enableDiego = func(appName string) {
			guid := getAppGuid(appName)
			Eventually(cf.Cf("curl", "/v2/apps/"+guid, "-X", "PUT", "-d", `{"diego": true}`), DEFAULT_TIMEOUT).Should(gexec.Exit(0))
		}

		By("checking if the expected non-tls VMs are running", func() {
			byteManifest, err := boshClient.DownloadManifest("cf")
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile("original-non-tls-manifest.yml", byteManifest, 0644)
			Expect(err).NotTo(HaveOccurred())

			nonTLSManifest = string(byteManifest)
			manifestName, err = ops.ManifestName(nonTLSManifest)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetNonErrandVMsFromManifest(nonTLSManifest)))
		})

		By("logging into cf and preparing the environment", func() {
			var boshVars map[string]interface{}
			err := yaml.Unmarshal(varsStoreBytes, &boshVars)
			Expect(err).NotTo(HaveOccurred())

			if _, ok := boshVars["uaa_scim_users_admin_password"]; !ok {
				Fail("Missing \"uaa_scim_users_admin_password\" key in vars store.")
			}

			cmd := exec.Command("cf", "login", "-a", fmt.Sprintf("api.%s", config.CF.Domain),
				"-u", "admin", "-p", boshVars["uaa_scim_users_admin_password"].(string),
				"--skip-ssl-validation")
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session, DEFAULT_TIMEOUT).Should(gexec.Exit(0))

			Eventually(cf.Cf("create-org", "EATS_org"), DEFAULT_TIMEOUT).Should(gexec.Exit(0))
			Eventually(cf.Cf("target", "-o", "EATS_org"), DEFAULT_TIMEOUT).Should(gexec.Exit(0))

			Eventually(cf.Cf("create-space", "EATS_space"), DEFAULT_TIMEOUT).Should(gexec.Exit(0))
			Eventually(cf.Cf("target", "-s", "EATS_space"), DEFAULT_TIMEOUT).Should(gexec.Exit(0))

			Eventually(cf.Cf("enable-feature-flag", "diego_docker"), DEFAULT_TIMEOUT).Should(gexec.Exit(0))
		})

		By("pushing an application to diego", func() {
			appName = generator.PrefixedRandomName("EATS-APP-", "")
			Eventually(cf.Cf(
				"push", appName,
				"-f", "assets/logspinner/manifest.yml",
				"--no-start"),
				CF_PUSH_TIMEOUT).Should(gexec.Exit(0))

			enableDiego(appName)

			Eventually(cf.Cf("start", appName), CF_PUSH_TIMEOUT).Should(gexec.Exit(0))
		})

		By("starting the syslog-drain process", func() {
			syslogAppName := generator.PrefixedRandomName("syslog-source-app-", "")
			Eventually(cf.Cf(
				"push", syslogAppName,
				"-f", "assets/logspinner/manifest.yml",
				"--no-start"),
				CF_PUSH_TIMEOUT).Should(gexec.Exit(0))

			enableDiego(syslogAppName)

			Eventually(cf.Cf("start", syslogAppName), CF_PUSH_TIMEOUT).Should(gexec.Exit(0))
			checker = syslogchecker.New("syslog-drainer", gen{}, 1*time.Millisecond, runner{})
			checker.Start(syslogAppName, fmt.Sprintf("http://%s.%s", syslogAppName, config.CF.Domain))
		})

		By("spamming logs", func() {
			consumer := consumer.New(fmt.Sprintf("wss://doppler.%s:443", config.CF.Domain), &tls.Config{InsecureSkipVerify: true}, nil)

			spammer = logspammer.NewSpammer(os.Stdout, fmt.Sprintf("http://%s.%s", appName, config.CF.Domain),
				func() (<-chan *events.Envelope, <-chan error) {
					return consumer.Stream(getAppGuid(appName), getToken())
				},
				100*time.Millisecond,
			)

			Eventually(func() bool {
				return spammer.CheckStream()
			}, "10s", "1s").Should(BeTrue())

			err = spammer.Start()
			Expect(err).NotTo(HaveOccurred())
		})

		By("deploying a TLS etcd cluster", func() {
			var err error
			etcdTLSManifest, err = addEtcdTLSInstanceGroup(nonTLSManifest, varsStore)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile("add-tls-etcd-deploy-manifest.yml", []byte(etcdTLSManifest), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(etcdTLSManifest))
			Expect(err).NotTo(HaveOccurred())
		})

		By("checking if the expected etcd-tls VMs are running", func() {
			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetNonErrandVMsFromManifest(etcdTLSManifest)))
		})

		By("scaling down the non-TLS etcd cluster to 1 node and converting it to a proxy", func() {
			var err error
			proxyTLSManifest, err = convertNonTLSEtcdToProxy(etcdTLSManifest, varsStore)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile("proxy-etcd-deploy-manifest.yml", []byte(proxyTLSManifest), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = boshClient.Deploy([]byte(proxyTLSManifest))
			Expect(err).NotTo(HaveOccurred())
		})

		By("checking if the expected proxy-tls VMs are running", func() {
			Eventually(func() ([]bosh.VM, error) {
				return helpers.DeploymentVMs(boshClient, manifestName)
			}, "1m", "10s").Should(ConsistOf(helpers.GetNonErrandVMsFromManifest(proxyTLSManifest)))
		})

		By("stopping spammer and checking for errors", func() {
			err = spammer.Stop()
			Expect(err).NotTo(HaveOccurred())

			spammerErrs, missingLogErrors := spammer.Check()

			var errorSet helpers.ErrorSet

			switch spammerErrs.(type) {
			case helpers.ErrorSet:
				errorSet = spammerErrs.(helpers.ErrorSet)
			case nil:
			default:
				Fail(spammerErrs.Error())
			}

			badGatewayErrCount := 0
			gatewayTimeoutErrCount := 0
			otherErrors := helpers.ErrorSet{}

			for err, occurrences := range errorSet {
				switch {
				// This typically happens when an active connection to a cell is interrupted during a cell evacuation
				case strings.Contains(err, "504 GATEWAY_TIMEOUT"):
					gatewayTimeoutErrCount += occurrences
				// This typically happens when an active connection to a cell is interrupted during a cell evacuation
				case strings.Contains(err, "502 Bad Gateway"):
					badGatewayErrCount += occurrences
				default:
					otherErrors.Add(errors.New(err))
				}
			}

			var missingLogErrorsCount int
			if missingLogErrors != nil {
				missingLogErrorsCount = len(missingLogErrors.(helpers.ErrorSet))
				if missingLogErrorsCount > MISSING_LOG_THRESHOLD {
					fmt.Println(missingLogErrors)
				}
			}

			Expect(otherErrors).To(HaveLen(0))
			Expect(missingLogErrorsCount).To(BeNumerically("<=", MISSING_LOG_THRESHOLD))
			Expect(gatewayTimeoutErrCount).To(BeNumerically("<=", GATEWAY_TIMEOUT_ERROR_COUNT_THRESHOLD))
			Expect(badGatewayErrCount).To(BeNumerically("<=", BAD_GATEWAY_ERROR_COUNT_THRESHOLD))
		})

		By("running a couple iterations of the syslog-drain checker", func() {
			count := checker.GetIterationCount()
			Eventually(checker.GetIterationCount, "10m", "10s").Should(BeNumerically(">", count+2))
		})

		By("stopping syslogchecker and checking for errors", func() {
			err = checker.Stop()
			Expect(err).NotTo(HaveOccurred())

			if ok, iterationCount, errPercent, errs := checker.Check(); ok {
				fmt.Println("total errors were within threshold")
				fmt.Println("total iterations:", iterationCount)
				fmt.Println("error percentage:", errPercent)
				fmt.Println(errs)
			} else {
				Fail(errs.Error())
			}
		})
	})
})
