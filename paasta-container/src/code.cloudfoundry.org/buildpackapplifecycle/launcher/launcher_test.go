package main_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Launcher", func() {
	var extractDir string
	var appDir string
	var launcherCmd *exec.Cmd
	var session *gexec.Session
	var startCommand string

	BeforeEach(func() {
		Expect(os.Setenv("CALLERENV", "some-value")).To(Succeed())

		if runtime.GOOS == "windows" {
			startCommand = "cmd /C set && echo PWD=%cd% && echo running app"
		} else {
			startCommand = "env; echo running app"
		}

		var err error
		extractDir, err = ioutil.TempDir("", "vcap")
		Expect(err).NotTo(HaveOccurred())

		appDir = filepath.Join(extractDir, "app")
		err = os.MkdirAll(appDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		launcherCmd = &exec.Cmd{
			Path: launcher,
			Dir:  extractDir,
			Env: append(
				os.Environ(),
				"TEST_CREDENTIAL_FILTER_WHITELIST=CALLERENV,DEPS_DIR,VCAP_APPLICATION,A,B,C,INSTANCE_GUID,INSTANCE_INDEX,PORT",
				"PORT=8080",
				"INSTANCE_GUID=some-instance-guid",
				"INSTANCE_INDEX=123",
				`VCAP_APPLICATION={"foo":1}`,
			),
		}
	})

	AfterEach(func() {
		err := os.RemoveAll(extractDir)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(launcherCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	var ItExecutesTheCommandWithTheRightEnvironment = func() {
		It("executes with the environment of the caller", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("CALLERENV=some-value"))
		})

		It("executes the start command with $HOME as the given dir", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("HOME=" + appDir))
		})

		It("changes to the app directory when running", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("PWD=" + appDir))
		})

		It("executes the start command with $TMPDIR as the extract directory + /tmp", func() {
			absDir, err := filepath.Abs(filepath.Join(appDir, "..", "tmp"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("TMPDIR=" + absDir))
		})

		It("executes the start command with $DEPS_DIR as the extract directory + /deps", func() {
			absDir, err := filepath.Abs(filepath.Join(appDir, "..", "deps"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("DEPS_DIR=" + absDir))
		})

		It("munges VCAP_APPLICATION appropriately", func() {
			Eventually(session).Should(gexec.Exit(0))

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

		Context("when the given dir has .profile.d with scripts in it", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip(".profile.d not supported on Windows")
				}

				var err error

				profileDir := filepath.Join(appDir, ".profile.d")

				err = os.MkdirAll(profileDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(profileDir, "a.sh"), []byte("echo sourcing a\nexport A=1\n"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(profileDir, "b.sh"), []byte("echo sourcing b\nexport B=1\n"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(appDir, ".profile"), []byte("echo sourcing .profile\nexport C=$A$B\n"), 0644)
				Expect(err).NotTo(HaveOccurred())

			})

			It("sources them before sourcing .profile and before executing", func() {
				Eventually(session).Should(gexec.Exit(0))
				Eventually(session).Should(gbytes.Say("sourcing a"))
				Eventually(session).Should(gbytes.Say("sourcing b"))
				Eventually(session).Should(gbytes.Say("sourcing .profile"))
				Eventually(session).Should(gbytes.Say("A=1"))
				Eventually(session).Should(gbytes.Say("B=1"))
				Eventually(session).Should(gbytes.Say("C=11"))
				Eventually(session).Should(gbytes.Say("running app"))
			})
		})
	}

	Context("the app executable is in vcap/app", func() {
		BeforeEach(func() {
			copyExe := func(dstDir, src string) error {
				in, err := os.Open(src)
				if err != nil {
					return err
				}
				defer in.Close()

				exeName := filepath.Base(src)
				dst := filepath.Join(dstDir, exeName)
				out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0755)
				if err != nil {
					return err
				}
				defer out.Close()
				_, err = io.Copy(out, in)
				cerr := out.Close()
				if err != nil {
					return err
				}
				return cerr
			}

			Expect(copyExe(appDir, hello)).To(Succeed())

			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"./hello",
				`{ "start_command": "echo should not run this" }`,
			}
		})

		It("finds the app executable", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
		})
	})

	Context("when a start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				startCommand,
				`{ "start_command": "echo should not run this" }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	var ItPrintsUsageInformation = func() {
		It("prints usage information", func() {
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say("Usage: launcher <app directory> <start command> <metadata>"))
		})
	}

	Context("when no start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				"",
			}
		})

		Context("when the app package does not contain staging_info.yml", func() {
			ItPrintsUsageInformation()
		})

		Context("when the app package has a staging_info.yml", func() {

			Context("when it is missing a start command", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "detected_buildpack: Ruby")
				})

				ItPrintsUsageInformation()
			})

			Context("when it contains a start command", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "detected_buildpack: Ruby\nstart_command: "+startCommand)
				})

				ItExecutesTheCommandWithTheRightEnvironment()
			})

			Context("when it references unresolvable types in non-essential fields", func() {
				BeforeEach(func() {
					writeStagingInfo(
						extractDir,
						"---\nbuildpack_path: !ruby/object:Pathname\n  path: /tmp/buildpacks/null-buildpack\ndetected_buildpack: \nstart_command: "+startCommand+"\n",
					)
				})

				ItExecutesTheCommandWithTheRightEnvironment()
			})

			Context("when it is not valid YAML", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "start_command: &ruby/object:Pathname")
				})

				It("prints an error message", func() {
					Eventually(session).Should(gexec.Exit(1))
					Eventually(session.Err).Should(gbytes.Say("Invalid staging info"))
				})
			})

		})

	})

	Context("when arguments are missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env",
			}
		})

		ItPrintsUsageInformation()
	})
})

func writeStagingInfo(extractDir, stagingInfo string) {
	err := ioutil.WriteFile(filepath.Join(extractDir, "staging_info.yml"), []byte(stagingInfo), 0644)
	Expect(err).NotTo(HaveOccurred())
}
