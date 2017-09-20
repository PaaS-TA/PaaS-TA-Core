package cell_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/ginkgoreporter"
	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/ifrit/grouper"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/inigo/gardenrunner"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/inigo_announcement_server"
	"code.cloudfoundry.org/inigo/world"
)

var (
	componentMaker world.ComponentMaker

	plumbing, bbsProcess ifrit.Process
	gardenClient         garden.Client
	bbsClient            bbs.InternalClient
	bbsServiceClient     bbs.ServiceClient
	logger               lager.Logger
)

var _ = SynchronizedBeforeSuite(func() []byte {
	payload, err := json.Marshal(world.BuiltArtifacts{
		Executables: CompileTestedExecutables(),
		Lifecycles:  BuildLifecycles(),
	})
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(encodedBuiltArtifacts []byte) {
	var builtArtifacts world.BuiltArtifacts

	err := json.Unmarshal(encodedBuiltArtifacts, &builtArtifacts)
	Expect(err).NotTo(HaveOccurred())

	localIP, err := localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	componentMaker = helpers.MakeComponentMaker(builtArtifacts, localIP)
})

var _ = BeforeEach(func() {
	plumbing = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
		{"etcd", componentMaker.Etcd()},
		{"sql", componentMaker.SQL()},
		{"nats", componentMaker.NATS()},
		{"consul", componentMaker.Consul()},
		{"garden", componentMaker.Garden()},
	}))
	bbsProcess = ginkgomon.Invoke(componentMaker.BBS())

	helpers.ConsulWaitUntilReady()
	logger = lager.NewLogger("test")
	logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

	gardenClient = componentMaker.GardenClient()
	bbsClient = componentMaker.BBSClient()
	bbsServiceClient = componentMaker.BBSServiceClient(logger)

	inigo_announcement_server.Start(componentMaker.ExternalAddress)
})

var _ = AfterEach(func() {
	inigo_announcement_server.Stop()

	destroyContainerErrors := helpers.CleanupGarden(gardenClient)

	helpers.StopProcesses(plumbing)

	Expect(destroyContainerErrors).To(
		BeEmpty(),
		"%d containers failed to be destroyed!",
		len(destroyContainerErrors),
	)
})

func TestCell(t *testing.T) {
	helpers.RegisterDefaultTimeouts()

	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t, "Cell Integration Suite", []Reporter{
		ginkgoreporter.New(GinkgoWriter),
	})
}

func CompileTestedExecutables() world.BuiltExecutables {
	var err error

	builtExecutables := world.BuiltExecutables{}

	builtExecutables["garden"], err = gexec.BuildIn(os.Getenv("GARDEN_GOPATH"), gardenrunner.GardenServerPackageName(), "-race", "-a", "-tags", "daemon")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["auctioneer"], err = gexec.BuildIn(os.Getenv("AUCTIONEER_GOPATH"), "code.cloudfoundry.org/auctioneer/cmd/auctioneer", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["rep"], err = gexec.BuildIn(os.Getenv("REP_GOPATH"), "code.cloudfoundry.org/rep/cmd/rep", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["bbs"], err = gexec.BuildIn(os.Getenv("BBS_GOPATH"), "code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["file-server"], err = gexec.BuildIn(os.Getenv("FILE_SERVER_GOPATH"), "code.cloudfoundry.org/fileserver/cmd/file-server", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["route-emitter"], err = gexec.BuildIn(os.Getenv("ROUTE_EMITTER_GOPATH"), "code.cloudfoundry.org/route-emitter/cmd/route-emitter", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["router"], err = gexec.BuildIn(os.Getenv("ROUTER_GOPATH"), "code.cloudfoundry.org/gorouter", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["ssh-proxy"], err = gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy", "-race")
	Expect(err).NotTo(HaveOccurred())

	os.Setenv("CGO_ENABLED", "0")
	builtExecutables["sshd"], err = gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-a", "-installsuffix", "static")
	os.Unsetenv("CGO_ENABLED")
	Expect(err).NotTo(HaveOccurred())

	return builtExecutables
}

func BuildLifecycles() world.BuiltLifecycles {
	builtLifecycles := world.BuiltLifecycles{}

	builderPath, err := gexec.BuildIn(os.Getenv("BUILDPACK_APP_LIFECYCLE_GOPATH"), "code.cloudfoundry.org/buildpackapplifecycle/builder", "-race")
	Expect(err).NotTo(HaveOccurred())

	launcherPath, err := gexec.BuildIn(os.Getenv("BUILDPACK_APP_LIFECYCLE_GOPATH"), "code.cloudfoundry.org/buildpackapplifecycle/launcher", "-race")
	Expect(err).NotTo(HaveOccurred())

	healthcheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck", "-race")
	Expect(err).NotTo(HaveOccurred())

	lifecycleDir, err := ioutil.TempDir("", "lifecycle-dir")
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(builderPath, filepath.Join(lifecycleDir, "builder"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(healthcheckPath, filepath.Join(lifecycleDir, "healthcheck"))
	Expect(err).NotTo(HaveOccurred())

	err = os.Rename(launcherPath, filepath.Join(lifecycleDir, "launcher"))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("tar", "-czf", "lifecycle.tar.gz", "builder", "launcher", "healthcheck")
	cmd.Stderr = GinkgoWriter
	cmd.Stdout = GinkgoWriter
	cmd.Dir = lifecycleDir
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	for _, stack := range helpers.PreloadedStacks {
		builtLifecycles[stack] = filepath.Join(lifecycleDir, "lifecycle.tar.gz")
	}

	return builtLifecycles
}
