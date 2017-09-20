package gardenrunner

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/onsi/ginkgo"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	Command *exec.Cmd

	network string
	addr    string

	bin  string
	argv []string

	binPath    string
	rootFSPath string

	tmpdir    string
	graphRoot string
	graphPath string
}

func UseOldGardenRunc() bool {
	// return true if we are using old garden-runc (i.e. version <= 0.4)
	// we use the package name to distinguish them
	oldGardenRuncPath := os.Getenv("GARDEN_GOPATH") + "/src/github.com/cloudfoundry-incubator/guardian"
	if _, err := os.Stat(oldGardenRuncPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func GardenServerPackageName() string {
	if UseOldGardenRunc() {
		return "github.com/cloudfoundry-incubator/guardian/cmd/guardian"
	}
	return "code.cloudfoundry.org/guardian/cmd/guardian"
}

func New(network, addr string, bin, binPath, rootFSPath, graphRoot string, argv ...string) *Runner {
	tmpDir := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("test-garden-%d", ginkgo.GinkgoParallelNode()),
	)

	if graphRoot == "" {
		graphRoot = filepath.Join(tmpDir, "graph")
	}

	graphPath := filepath.Join(graphRoot, fmt.Sprintf("node-%d", ginkgo.GinkgoParallelNode()))

	return &Runner{
		network: network,
		addr:    addr,

		bin:  bin,
		argv: argv,

		binPath:    binPath,
		rootFSPath: rootFSPath,
		graphRoot:  graphRoot,
		graphPath:  graphPath,
		tmpdir:     tmpDir,
	}
}

func (r *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := lagertest.NewTestLogger("garden-runner")

	if err := os.MkdirAll(r.tmpdir, 0755); err != nil {
		return err
	}

	depotPath := filepath.Join(r.tmpdir, "containers")
	snapshotsPath := filepath.Join(r.tmpdir, "snapshots")

	if err := os.MkdirAll(depotPath, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(snapshotsPath, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(r.graphPath, 0755); err != nil {
		return err
	}

	var appendDefaultFlag = func(ar []string, key, value string) []string {
		for _, a := range r.argv {
			if a == key {
				return ar
			}
		}

		if value != "" {
			return append(ar, key, value)
		} else {
			return append(ar, key)
		}
	}

	gardenArgs := make([]string, len(r.argv))
	copy(gardenArgs, r.argv)

	gardenArgs = appendDefaultFlag(gardenArgs, "--depot", depotPath)
	gardenArgs = appendDefaultFlag(gardenArgs, "--graph", r.graphPath)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tag", strconv.Itoa(ginkgo.GinkgoParallelNode()))

	if UseOldGardenRunc() {
		gardenArgs = appendDefaultFlag(gardenArgs, "--iodaemon-bin", r.binPath+"/iodaemon")
		gardenArgs = appendDefaultFlag(gardenArgs, "--kawasaki-bin", r.binPath+"/kawasaki")
	}

	gardenArgs = appendDefaultFlag(gardenArgs, "--init-bin", r.binPath+"/init")
	gardenArgs = appendDefaultFlag(gardenArgs, "--dadoo-bin", r.binPath+"/dadoo")
	gardenArgs = appendDefaultFlag(gardenArgs, "--nstar-bin", r.binPath+"/nstar")
	gardenArgs = appendDefaultFlag(gardenArgs, "--tar-bin", r.binPath+"/tar")
	gardenArgs = appendDefaultFlag(gardenArgs, "--runc-bin", r.binPath+"/runc")
	gardenArgs = appendDefaultFlag(gardenArgs, "--port-pool-start", strconv.Itoa(51000+(1000*ginkgo.GinkgoParallelNode())))
	gardenArgs = appendDefaultFlag(gardenArgs, "--port-pool-size", "1000")

	switch r.network {
	case "tcp":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-ip", strings.Split(r.addr, ":")[0])
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-port", strings.Split(r.addr, ":")[1])
	case "unix":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-socket", r.addr)
	}

	gardenArgs = appendDefaultFlag(gardenArgs, "--network-pool", fmt.Sprintf("10.250.%d.0/24", ginkgo.GinkgoParallelNode()))

	if r.rootFSPath != "" { //default-rootfs is an optional parameter
		gardenArgs = appendDefaultFlag(gardenArgs, "--default-rootfs", r.rootFSPath)
	}

	gardenArgs = appendDefaultFlag(gardenArgs, "--allow-host-access", "")

	var signal os.Signal

	r.Command = exec.Command(r.bin, gardenArgs...)

	process := ifrit.Invoke(&ginkgomon.Runner{
		Name:              "garden",
		Command:           r.Command,
		AnsiColorCode:     "31m",
		StartCheck:        "guardian.started",
		StartCheckTimeout: 30 * time.Second,
		Cleanup: func() {
			if signal == syscall.SIGQUIT {
				logger.Info("cleanup-subvolumes")

				// remove contents of subvolumes before deleting the subvolume
				if err := os.RemoveAll(r.graphPath); err != nil {
					logger.Error("remove graph", err)
				}

				logger.Info("cleanup-tempdirs")
				if err := os.RemoveAll(r.tmpdir); err != nil {
					logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.tmpdir})
				} else {
					logger.Info("tempdirs-removed")
				}
			}
		},
	})

	close(ready)

	for {
		select {
		case signal = <-signals:
			// SIGQUIT means clean up the containers, the garden process (SIGTERM) and the temporary directories
			// SIGKILL, SIGTERM and SIGINT are passed through to the garden process
			if signal == syscall.SIGQUIT {
				logger.Info("received-signal SIGQUIT")
				if err := r.destroyContainers(); err != nil {
					logger.Error("destroy-containers-failed", err)
					return err
				}
				logger.Info("destroyed-containers")
				process.Signal(syscall.SIGTERM)
			} else {
				logger.Info("received-signal", lager.Data{"signal": signal})
				process.Signal(signal)
			}

		case waitErr := <-process.Wait():
			logger.Info("process-exited")
			return waitErr
		}
	}
}

func (r *Runner) TryDial() error {
	conn, dialErr := net.DialTimeout(r.network, r.addr, 100*time.Millisecond)

	if dialErr == nil {
		conn.Close()
		return nil
	}

	return dialErr
}

func (r *Runner) NewClient() client.Client {
	return client.New(connection.New(r.network, r.addr))
}

func (r *Runner) destroyContainers() error {
	client := r.NewClient()

	containers, err := client.Containers(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := client.Destroy(container.Handle())
		if err != nil {
			return err
		}
	}

	return nil
}
