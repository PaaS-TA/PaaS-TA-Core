package upgrade_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Upgrade Stability Tests", func() {
	var (
		sess          *Session
		err           error
		pollerProcess ifrit.Process
		pollerApp     *cfApp
	)

	BeforeEach(func() {
		By("Deploying V0")
		By("Deleting existing deployments")
		boshCmd("", "delete deployment cf-warden", "")
		boshCmd("", "delete deployment cf-warden-diego-database", "")
		boshCmd("", "delete deployment cf-warden-diego-brain-and-pals", "")
		boshCmd("", "delete deployment cf-warden-diego-cell1 --force", "")
		boshCmd("", "delete deployment cf-warden-diego-cell2 --force", "")
		boshCmd("", "delete deployment cf-warden-mysql --force", "")

		By("Ensuring the V0 is not currently deployed")
		deploymentsCmd := bosh("deployments")
		sess, err = Start(deploymentsCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, COMMAND_TIMEOUT).Should(Exit())
		Expect(sess).NotTo(Say("cf-warden"))
		Expect(sess).NotTo(Say("cf-warden-diego-brain-and-pals"))
		Expect(sess).NotTo(Say("cf-warden-diego-cell1"))
		Expect(sess).NotTo(Say("cf-warden-diego-cell2"))
		Expect(sess).NotTo(Say("cf-warden-diego-database"))
		Expect(sess).NotTo(Say("cf-warden-mysql"))

		By("Generating the V0 deployment manifests for 5 piece wise deployments")
		arguments := []string{
			"-d", filepath.Join(config.BaseReleaseDirectory, config.V0DiegoReleasePath),
			"-c", filepath.Join(config.BaseReleaseDirectory, config.V0CfReleasePath),
			"-a", filepath.Join(config.BaseReleaseDirectory, config.AwsStubsDirectory),
			"-o", config.OverrideDomain, // Leave the -o option last. getops exits in script if this is empty
			"-z",
		}

		if config.DiegoReleaseV0Legacy {
			arguments = append(arguments, "-l")
		}

		if config.UseSQLV0 {
			arguments = append(arguments, "-s")
		}

		generateManifestCmd := exec.Command("./scripts/generate-manifests", arguments...)

		generateManifestCmd.Env = append(
			os.Environ(),
			fmt.Sprintf("BOSH_RELEASES_DIR:%s", filepath.Join(config.BaseReleaseDirectory, "releases")),
		)

		sess, err = Start(generateManifestCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, COMMAND_TIMEOUT).Should(Exit(0))

		if config.UseSQLV0 || config.UseSQLVPrime {
			arguments = []string{
				"-d", filepath.Join(config.BaseReleaseDirectory, config.V1DiegoReleasePath),
				"-c", filepath.Join(config.BaseReleaseDirectory, config.V1CfReleasePath),
			}
			generateMySqlManifestCmd := exec.Command("./scripts/generate-mysql-manifest", arguments...)
			sess, err = Start(generateMySqlManifestCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, COMMAND_TIMEOUT).Should(Exit(0))
		}

		By("Deploying CF")
		boshCmd("manifests/cf.yml", "deploy", "Deployed 'cf-warden'")

		if config.UseSQLV0 {
			By("Deploying MySQL")
			boshCmd("manifests/cf-mysql.yml", "deploy", "Deployed 'cf-warden-mysql'")
		}

		By("Deploying Database")
		boshCmd("manifests/database.yml", "deploy", "Deployed 'cf-warden-diego-database'")

		By("Deploying Brain and Pals")
		boshCmd("manifests/brain-and-pals.yml", "deploy", "Deployed 'cf-warden-diego-brain-and-pals'")

		By("Deploying Cell 1")
		boshCmd("manifests/cell1.yml", "deploy", "Deployed 'cf-warden-diego-cell1'")

		By("Deploying Cell 2")
		boshCmd("manifests/cell2.yml", "deploy", "Deployed 'cf-warden-diego-cell2'")

		By("Deploying a Test App")
		pollerApp = newCfApp("test-app", config.MaxPollingErrors)
		pollerApp.Push()

		By("Continuously Polling the Test Application")
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())
	})

	AfterEach(func() {
		By("Test Complete, AfterEach Beginning")
		ginkgomon.Kill(pollerProcess)

		By("Deleting the Test App")
		pollerApp.Destroy()
	})

	It("Upgrades from V0 to V1", func() {
		By("Generating the V1 deployment manifests for 5 piece wise deployments")
		arguments := []string{
			"-d", filepath.Join(config.BaseReleaseDirectory, config.V1DiegoReleasePath),
			"-c", filepath.Join(config.BaseReleaseDirectory, config.V1CfReleasePath),
			"-a", filepath.Join(config.BaseReleaseDirectory, config.AwsStubsDirectory),
			"-o", config.OverrideDomain,
		}

		if config.UseSQLVPrime {
			arguments = append(arguments, "-s")
		}

		generateManifestCmd := exec.Command("./scripts/generate-manifests", arguments...)

		sess, err := Start(generateManifestCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, COMMAND_TIMEOUT).Should(Exit(0))

		// only deploy mysql if it wasn't deployed in V0
		if config.UseSQLVPrime && !config.UseSQLV0 {
			By("Deploying MySQL")
			boshCmd("manifests/cf-mysql.yml", "deploy", "Deployed 'cf-warden-mysql'")
		}

		// Roll CF. This must happen first as it configures ETCD to talk over
		// TLS which is backwards compaitble with older clients, but not the
		// other way around.
		// ************************************************************ //
		// UPGRADE CF
		By("Upgrading CF")
		ginkgomon.Kill(pollerProcess)
		boshCmd("manifests/cf.yml", "deploy", "Deployed 'cf-warden'")
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())

		By("Running Smoke Tests #1")
		smokeTestDiego()

		By("Scaling Test App #1")
		pollerApp.Scale(2)
		pollerApp.Scale(1)

		// Roll the Diego Database
		// ************************************************************ //
		// UPGRADE D1
		By("Upgrading Database")
		boshCmd("manifests/database.yml", "deploy", "Deployed 'cf-warden-diego-database'")

		By("Running Smoke Tests #2")
		smokeTestDiego()

		By("Scaling Test App #2")
		pollerApp.Scale(2)
		pollerApp.Scale(1)

		// Rolling some cells, and turning off the other in order to
		// test the new database, new cells, old brain and CF
		// ************************************************************ //
		// UPGRADE D3
		By("Upgrading Cell 1")
		boshCmd("manifests/cell1.yml", "deploy --recreate", "Deployed 'cf-warden-diego-cell1'")
		ginkgomon.Kill(pollerProcess)
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())

		// AFTER UPGRADING D3, PRESERVE OLD DEPLOYMENT AND STOP D4
		// Deleting the deployment because #108279564
		By("Stopping Cell 2")
		boshCmd("", "download manifest cf-warden-diego-cell2 manifests/legacy-cell-2.yml", `Deployment manifest saved to .manifests\/legacy-cell-2.yml'`)
		boshCmd("manifests/legacy-cell-2.yml", "stop cell_z2", `cell_z2\/.* stopped, VM\(s\) still running`)
		boshCmd("manifests/legacy-cell-2.yml", "delete deployment cf-warden-diego-cell2 --force", "Deleted deployment 'cf-warden-diego-cell2'")
		ginkgomon.Kill(pollerProcess)
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())

		By("Running Smoke Tests #3")
		smokeTestDiego()

		By("Scaling Test App #3")
		pollerApp.Scale(2)
		pollerApp.Scale(1)

		// Rolling the Brain, but turning off the new cells and turning back on
		// the old cells to test when everything on diego has rolled except the cells.
		// ************************************************************ //
		// UPGRADE D2
		By("Upgrading Brain and Pals")
		boshCmd("manifests/brain-and-pals.yml", "deploy", "Deployed 'cf-warden-diego-brain-and-pals'")

		// START D4
		By("Deploying Cell 2")
		boshCmd("manifests/legacy-cell-2.yml", "deploy", "Deployed 'cf-warden-diego-cell2'")

		// AND STOP D3
		// Deleting the deployment because #108279564
		By("Stopping Cell 1")
		boshCmd("manifests/cell1.yml", "stop cell_z1", `cell_z1\/.* stopped, VM\(s\) still running`)
		boshCmd("manifests/cell1.yml", "delete deployment cf-warden-diego-cell1 --force", "Deleted deployment 'cf-warden-diego-cell1'")
		ginkgomon.Kill(pollerProcess)
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())

		By("Running Smoke Tests #4")
		smokeTestDiego()

		By("Scaling Test App #4")
		pollerApp.Scale(2)
		pollerApp.Scale(1)

		// Roll the rest of the cells and test that everything is now stable at the
		// new deployment.
		// ************************************************************ //
		// BEFORE UPGRADING D4, START D3
		By("Starting Cell 1")
		boshCmd("manifests/cell1.yml", "deploy", "Deployed 'cf-warden-diego-cell1'")

		// UPGRADE D4
		By("Upgrading Cell 2")
		boshCmd("manifests/cell2.yml", "deploy --recreate", "Deployed 'cf-warden-diego-cell2'")
		ginkgomon.Kill(pollerProcess)
		pollerProcess = ginkgomon.Invoke(pollerApp.NewPoller())

		By("Running Smoke Tests #5")
		smokeTestDiego()

		By("Scaling Test App #5")
		pollerApp.Scale(2)
		pollerApp.Scale(1)
	})
})
