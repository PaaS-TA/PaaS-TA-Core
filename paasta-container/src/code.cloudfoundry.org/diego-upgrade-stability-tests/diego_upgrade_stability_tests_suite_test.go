package upgrade_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const BOSH_DEPLOY_TIMEOUT = 60 * time.Minute
const COMMAND_TIMEOUT = 60 * time.Second

var config *TestConfig

type TestConfig struct {
	BoshDirectorURL   string `json:"bosh_director_url"`
	BoshAdminUser     string `json:"bosh_admin_user"`
	BoshAdminPassword string `json:"bosh_admin_password"`

	BaseReleaseDirectory string `json:"base_directory"`
	V0DiegoReleasePath   string `json:"v0_diego_release_path"`
	V0CfReleasePath      string `json:"v0_cf_release_path"`
	V1DiegoReleasePath   string `json:"v1_diego_release_path"`
	V1CfReleasePath      string `json:"v1_cf_release_path"`
	OverrideDomain       string `json:"override_domain"`
	MaxPollingErrors     int    `json:"max_polling_errors,omitempty"`
	UseSQLV0             bool   `json:"use_sql_v0"`

	DiegoReleaseV0Legacy bool `json:"diego_release_v0_legacy"`

	AwsStubsDirectory string `json:"aws_stubs_directory"`
}

func bosh(args ...string) *exec.Cmd {
	boshArgs := append([]string{"-t", config.BoshDirectorURL, "-u", config.BoshAdminUser, "-p", config.BoshAdminPassword}, args...)
	return exec.Command("bosh", boshArgs...)
}

func TestUpgradeStableManifests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "UpgradeStableManifests Suite")
}

var _ = BeforeSuite(func() {
	config = &TestConfig{
		OverrideDomain:   "bosh-lite.com",
		MaxPollingErrors: 1,
	}

	SetDefaultEventuallyTimeout(time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	path := os.Getenv("CONFIG")
	Expect(path).NotTo(Equal(""))

	configFile, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(config)
	Expect(err).NotTo(HaveOccurred())

	boshTargetCmd := bosh("target", config.BoshDirectorURL)
	sess, err := gexec.Start(boshTargetCmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, COMMAND_TIMEOUT).Should(gexec.Exit(0))
})
