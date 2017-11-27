package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Launcher", func() {
	var (
		appDir      string
		launcherCmd *exec.Cmd
		session     *gexec.Session
		workdir     string
	)

	BeforeEach(func() {
		os.Setenv("CALLERENV", "some-value")

		var err error
		appDir, err = ioutil.TempDir("", "app-dir")
		Expect(err).NotTo(HaveOccurred())

		workdir = "/"

		launcherCmd = &exec.Cmd{
			Path: launcher,
			Env: append(
				os.Environ(),
				"PORT=8080",
				"INSTANCE_GUID=some-instance-guid",
				"INSTANCE_INDEX=123",
				`VCAP_APPLICATION={"foo":1}`,
			),
		}
	})

	AfterEach(func() {
		os.RemoveAll(appDir)
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(launcherCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	var ItExecutesTheCommandWithTheRightEnvironment = func() {
		It("executes the start command", func() {
			Eventually(session).Should(gbytes.Say("running app"))
		})

		It("executes with the environment of the caller", func() {
			Eventually(session).Should(gbytes.Say("CALLERENV=some-value"))
		})

		It("changes to the workdir when running", func() {
			// wildcard because PWD expands symlinks and appDir temp folder might be one
			Eventually(session).Should(gbytes.Say("PWD=" + workdir + "\n"))
		})

		It("munges VCAP_APPLICATION appropriately", func() {
			Eventually(session, 3, "100ms").Should(gexec.Exit(0))

			vcapAppPattern := regexp.MustCompile("VCAP_APPLICATION=(.*)")
			vcapApplicationBytes := vcapAppPattern.FindSubmatch(session.Out.Contents())[1]

			vcapApplication := map[string]interface{}{}
			err := json.Unmarshal(vcapApplicationBytes, &vcapApplication)
			Expect(err).NotTo(HaveOccurred())

			Expect(vcapApplication["host"]).To(Equal("0.0.0.0"))
			Expect(vcapApplication["port"]).To(Equal(float64(8080)))
			Expect(vcapApplication["instance_index"]).To(Equal(float64(123)))
			Expect(vcapApplication["instance_id"]).To(Equal("some-instance-guid"))
			Expect(vcapApplication["foo"]).To(Equal(float64(1)))
		})
	}

	Context("when a start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env; echo running app",
				`{ "cmd": ["echo should not run this"] }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Context("when a start command is given with a workdir", func() {
		BeforeEach(func() {
			workdir = "/bin"
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env; echo running app",
				fmt.Sprintf(`{ "cmd" : ["echo should not run this"],
				   "workdir" : "%s"}`, workdir),
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Context("when no start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "cmd": ["/bin/sh", "-c", "env; echo running app"] }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Context("when both an entrypoint and a cmd are in the metadata", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "entrypoint": ["/bin/echo"], "cmd": ["abc"] }`,
			}
		})

		It("includes the entrypoint before the cmd args", func() {
			Eventually(session).Should(gbytes.Say("abc"))
		})
	})

	Context("when an entrypoint, a cmd, and a workdir are all in the metadata", func() {
		BeforeEach(func() {
			workdir = "/bin"
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				fmt.Sprintf(`{ "entrypoint": ["./echo"], "cmd": ["abc"], "workdir" : "%s"}`, workdir),
			}
		})

		It("runs the composite command in the workdir", func() {
			Eventually(session).Should(gbytes.Say("abc"))
		})
	})

	Context("when no start command or execution metadata is present", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{}`,
			}
		})

		It("errors", func() {
			Eventually(session.Err).Should(gbytes.Say("No start command found or specified"))
		})
	})

	ItPrintsUsageInformation := func() {
		It("prints usage information", func() {
			Eventually(session.Err).Should(gbytes.Say("Usage: launcher <ignored> <start command> <metadata>"))
			Eventually(session).Should(gexec.Exit(1))
		})
	}

	Context("when no arguments are given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the start command and metadata are missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the metadata is missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env",
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the given execution metadata is not valid JSON", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				"{ not-valid-json }",
			}
		})

		It("prints an error message", func() {
			Eventually(session.Err).Should(gbytes.Say("Invalid metadata"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when no start command is given, and exec fails", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "cmd": ["/bin/sh", "-c", "exit 9"] }`,
			}
		})

		It("correctly bubbles non-zero exit codes", func() {
			Eventually(session).Should(gexec.Exit(9))
		})
	})
})
