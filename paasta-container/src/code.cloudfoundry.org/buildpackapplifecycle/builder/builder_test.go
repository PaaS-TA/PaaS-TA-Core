package main_test

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Building", func() {

	var (
		builderCmd *exec.Cmd

		tmpDir                    string
		buildDir                  string
		buildpacksDir             string
		outputDroplet             string
		buildpackOrder            string
		buildArtifactsCacheDir    string
		outputMetadata            string
		outputBuildArtifactsCache string
		skipDetect                bool
	)

	builder := func() *gexec.Session {
		session, err := gexec.Start(
			builderCmd,
			GinkgoWriter,
			GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())

		return session
	}

	cpBuildpack := func(buildpack string) {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(buildpack)))
		cp(filepath.Join(buildpackFixtures, buildpack), filepath.Join(buildpacksDir, hash))
	}

	BeforeEach(func() {
		var err error

		tmpDir, err = ioutil.TempDir("", "building-tmp")
		buildDir, err = ioutil.TempDir(tmpDir, "building-app")
		Expect(err).NotTo(HaveOccurred())

		buildpacksDir, err = ioutil.TempDir(tmpDir, "building-buildpacks")
		Expect(err).NotTo(HaveOccurred())

		outputDropletFile, err := ioutil.TempFile(tmpDir, "building-droplet")
		Expect(err).NotTo(HaveOccurred())
		outputDroplet = outputDropletFile.Name()
		Expect(outputDropletFile.Close()).To(Succeed())

		outputBuildArtifactsCacheDir, err := ioutil.TempDir(tmpDir, "building-cache-output")
		Expect(err).NotTo(HaveOccurred())
		outputBuildArtifactsCache = filepath.Join(outputBuildArtifactsCacheDir, "cache.tgz")

		buildArtifactsCacheDir, err = ioutil.TempDir(tmpDir, "building-cache")
		Expect(err).NotTo(HaveOccurred())

		outputMetadataFile, err := ioutil.TempFile(tmpDir, "building-result")
		Expect(err).NotTo(HaveOccurred())
		outputMetadata = outputMetadataFile.Name()
		Expect(outputMetadataFile.Close()).To(Succeed())

		buildpackOrder = ""

		skipDetect = false
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		builderCmd = exec.Command(builderPath,
			"-buildDir", buildDir,
			"-buildpacksDir", buildpacksDir,
			"-outputDroplet", outputDroplet,
			"-outputBuildArtifactsCache", outputBuildArtifactsCache,
			"-buildArtifactsCacheDir", buildArtifactsCacheDir,
			"-buildpackOrder", buildpackOrder,
			"-outputMetadata", outputMetadata,
			"-skipDetect="+strconv.FormatBool(skipDetect),
		)

		env := os.Environ()
		builderCmd.Env = append(env, "TMPDIR="+tmpDir)
	})

	resultJSON := func() []byte {
		resultInfo, err := ioutil.ReadFile(outputMetadata)
		Expect(err).NotTo(HaveOccurred())

		return resultInfo
	}

	Context("run detect", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects,also-always-detects"

			cpBuildpack("always-detects")
			cpBuildpack("also-always-detects")
			cp(path.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		JustBeforeEach(func() {
			Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
		})

		Describe("the contents of the output tgz", func() {
			var files []string

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputDroplet).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			It("should contain an /app dir with the contents of the compilation", func() {
				Expect(files).To(ContainElement("./app/"))
				Expect(files).To(ContainElement("./app/app.sh"))
				Expect(files).To(ContainElement("./app/compiled"))
			})

			It("should contain an empty /tmp directory", func() {
				Expect(files).To(ContainElement("./tmp/"))
				Expect(files).NotTo(ContainElement(MatchRegexp("\\./tmp/.+")))
			})

			It("should contain an empty /logs directory", func() {
				Expect(files).To(ContainElement("./logs/"))
				Expect(files).NotTo(ContainElement(MatchRegexp("\\./logs/.+")))
			})

			It("should contain a staging_info.yml with the detected buildpack", func() {
				stagingInfo, err := exec.Command("tar", "-xzf", outputDroplet, "-O", "./staging_info.yml").Output()
				Expect(err).NotTo(HaveOccurred())

				expectedYAML := `{"detected_buildpack":"Always Matching","start_command":"the start command"}`
				Expect(string(stagingInfo)).To(MatchJSON(expectedYAML))
			})

			Context("buildpack with supply/finalize", func() {
				BeforeEach(func() {
					buildpackOrder = "has-finalize,always-detects,also-always-detects"
					cpBuildpack("has-finalize")
				})

				It("runs supply/finalize and not compile", func() {
					Expect(files).To(ContainElement("./app/finalized"))
					Expect(files).ToNot(ContainElement("./app/compiled"))
				})
			})
		})

		Describe("the build artifacts cache output tgz", func() {
			BeforeEach(func() {
				buildpackOrder = "always-detects-creates-build-artifacts"

				cpBuildpack("always-detects-creates-build-artifacts")
			})

			It("gets created", func() {
				result, err := exec.Command("tar", "-tzf", outputBuildArtifactsCache).Output()
				Expect(err).NotTo(HaveOccurred())

				Expect(removeTrailingSpace(strings.Split(string(result), "\n"))).To(ContainElement("./final/build-artifact"))
			})
		})

		Describe("the result.json, which is used to communicate back to the stager", func() {
			It("exists, and contains the detected buildpack", func() {
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
				}`))
			})

			Context("when the app has a Procfile", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("uses the Procfile processes in the execution metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
				 }`))
				})
			})

			Context("when the app does not have a Procfile", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("uses the default_process_types specified by the buildpack", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
				 }`))
				})
			})
		})
	})

	Context("skip detect", func() {
		BeforeEach(func() {
			skipDetect = true
		})

		JustBeforeEach(func() {
			Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
		})

		Describe("the contents of the output tgz", func() {
			var files []string

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputDroplet).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			Describe("the result.json, which is used to communicate back to the stager", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects"
					cpBuildpack("always-detects")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})
				It("exists, and contains the final buildpack key", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
				}`))
				})
			})

			Context("final buildpack does not contain a finalize script", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,also-always-detects"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("also-always-detects")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").CombinedOutput()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					Expect(files).ToNot(ContainElement("./deps/2/supplied"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/compiled"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/compiled").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("also-always-detects-buildpack"))
				})

				It("the /deps dir is not passed to the final compile command", func() {
					Expect(files).ToNot(ContainElement("./deps/compiled"))
				})
			})

			Context("final buildpack contains finalize + supply scripts", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/2/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/finalized"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/finalized").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
				})
			})

			Context("final buildpack only contains finalize ", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize-no-supply"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize-no-supply")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					Expect(files).ToNot(ContainElement("./deps/2/supplied"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/finalized"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/finalized").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-no-supply-buildpack"))
				})
			})

			Context("buildpack that fails detect", func() {
				BeforeEach(func() {
					buildpackOrder = "always-fails-detect"

					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
					cpBuildpack("always-fails-detect")
				})

				It("should run successfully", func() {
					Expect(files).To(ContainElement("./app/compiled"))
				})
			})
		})

		Describe("the contents of the cache tgz", func() {
			var files []string

			BeforeEach(func() {
				buildpackOrder = "always-detects-creates-build-artifacts,always-detects,also-always-detects"

				cpBuildpack("always-detects-creates-build-artifacts")
				cpBuildpack("always-detects")
				cpBuildpack("also-always-detects")
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputBuildArtifactsCache).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			Context("final buildpack does not contain finalize", func() {
				Describe("the buildArtifactsCacheDir is empty", func() {
					It("the final buildpack caches compile output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/compiled"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/compiled").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("also-always-detects-buildpack"))
					})

					It("the supply buildpacks caches supply output as $CACHE_DIR/<md5sum of buildpack URL>", func() {
						supplyCacheDir := fmt.Sprintf("%x", md5.Sum([]byte("always-detects-creates-build-artifacts")))
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						supplyCacheDir = fmt.Sprintf("%x", md5.Sum([]byte("always-detects")))
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+supplyCacheDir+"/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))
					})
				})
			})

			Context("final buildpack contains finalize", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				Describe("the buildArtifactsCacheDir is empty", func() {
					It("the final buildpack caches finalize output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/finalized"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/finalized").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
					})

					It("the final buildpack caches supply output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
					})

					It("the supply buildpacks caches supply output as $CACHE_DIR/<md5sum of buildpack URL>", func() {
						supplyCacheDir := fmt.Sprintf("%x", md5.Sum([]byte("always-detects-creates-build-artifacts")))
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						supplyCacheDir = fmt.Sprintf("%x", md5.Sum([]byte("always-detects")))
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+supplyCacheDir+"/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))
					})
				})
			})

			Describe("the buildArtifactsCacheDir contains relevant and old buildpack cache directories", func() {
				//test setup
				var (
					alwaysDetectsMD5       string
					notInBuildpackOrderMD5 string
					cachedSupply           string
					cachedCompile          string
				)

				BeforeEach(func() {
					rand.Seed(time.Now().UnixNano())
					cachedSupply = fmt.Sprintf("%d", rand.Int())
					alwaysDetectsMD5 = fmt.Sprintf("%x", md5.Sum([]byte("always-detects")))
					err := os.MkdirAll(filepath.Join(buildArtifactsCacheDir, alwaysDetectsMD5), 0755)
					Expect(err).To(BeNil())
					err = ioutil.WriteFile(filepath.Join(buildArtifactsCacheDir, alwaysDetectsMD5, "old-supply"), []byte(cachedSupply), 0644)
					Expect(err).To(BeNil())

					notInBuildpackOrderMD5 = fmt.Sprintf("%x", md5.Sum([]byte("not-in-buildpack-order")))
					err = os.MkdirAll(filepath.Join(buildArtifactsCacheDir, notInBuildpackOrderMD5), 0755)
					Expect(err).To(BeNil())

					cachedCompile = fmt.Sprintf("%d", rand.Int())
					err = os.MkdirAll(filepath.Join(buildArtifactsCacheDir, "final"), 0755)
					Expect(err).To(BeNil())
					err = ioutil.WriteFile(filepath.Join(buildArtifactsCacheDir, "final", "old-compile"), []byte(cachedCompile), 0644)
					Expect(err).To(BeNil())

					err = ioutil.WriteFile(filepath.Join(buildArtifactsCacheDir, "pre-multi-file"), []byte("Some Content"), 0644)
					Expect(err).To(BeNil())
				})

				It("does not remove the cached contents of $CACHE_DIR/final", func() {
					Expect(files).To(ContainElement("./final/compiled"))

					content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/compiled").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal(cachedCompile))
				})

				It("does not remove the cached contents of buildpacks in buildpack order", func() {
					Expect(files).To(ContainElement("./" + alwaysDetectsMD5 + "/supplied"))

					content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+alwaysDetectsMD5+"/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal(cachedSupply))
				})

				It("removes the cached contents of buildpacks not in buildpack order", func() {
					Expect(files).NotTo(ContainElement("./" + notInBuildpackOrderMD5 + "/"))
				})

				It("removes any files from pre multi buildpack days", func() {
					Expect(files).NotTo(ContainElement("./pre-multi-file"))
				})
			})
		})
	})

	Context("with a buildpack that has no commands", func() {
		BeforeEach(func() {
			buildpackOrder = "release-without-command"
			cpBuildpack("release-without-command")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("uses the Procfile for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Release Without Command",
							"buildpack_key": "release-without-command"
						},
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("displays an error and returns the Procfile data without web", func() {
					session := builder()
					Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
					Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
					Eventually(session).Should(gexec.Exit(0))

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Release Without Command",
							"buildpack_key": "release-without-command"
						},
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {
			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("fails", func() {
				session := builder()
				Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
				Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})

	Context("with a buildpack that determines a start web-command", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects"
			cpBuildpack("always-detects")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("merges the Procfile and the buildpack for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("merges the Procfile but uses the buildpack for execution_metadata", func() {

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command", "web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {

			JustBeforeEach(func() {
				Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
			})

			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("merges the Procfile and the buildpack for execution_metadata", func() {
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects"
						},
						"execution_metadata": ""
					}`))
			})
		})
	})

	Context("with a buildpack that determines a start non-web-command", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects-non-web"
			cpBuildpack("always-detects-non-web")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("merges the Procfile for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command", "nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web"
						},
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("displays an error and returns the Procfile data without web", func() {
					session := builder()
					Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
					Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
					Eventually(session).Should(gexec.Exit(0))

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command", "nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web"
						},
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {
			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("fails", func() {
				session := builder()
				Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
				Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
				Eventually(session).Should(gexec.Exit(0))
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web"
						},
						"execution_metadata": ""
					}`))
			})
		})
	})

	Context("with an app with an invalid Procfile", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects,also-always-detects"

			cpBuildpack("always-detects")
			cpBuildpack("also-always-detects")

			cp(filepath.Join(appFixtures, "bogus-procfile", "Procfile"), buildDir)
		})

		It("fails", func() {
			session := builder()
			Eventually(session.Err).Should(gbytes.Say("Failed to read command from Procfile: invalid YAML"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when no buildpacks match", func() {
		BeforeEach(func() {
			buildpackOrder = "always-fails-detect"

			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			cpBuildpack("always-fails-detect")
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session).Should(gexec.Exit(222))
			Expect(session.Err).To(gbytes.Say("None of the buildpacks detected a compatible application"))
		})
	})

	Context("when the buildpack fails in compile", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-compile"

			cpBuildpack("fails-to-compile")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session).Should(gexec.Exit(223))
			Expect(session.Err).Should(gbytes.Say("Failed to compile droplet"))
		})
	})

	Context("when a buildpack fails a supply script", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-supply,always-detects"
			skipDetect = true

			cpBuildpack("fails-to-supply")
			cpBuildpack("always-detects")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session).Should(gexec.Exit(225))
			Expect(session.Err).Should(gbytes.Say("Failed to run all supply scripts"))
		})
	})

	Context("when the buildpack release generates invalid yaml", func() {
		BeforeEach(func() {
			buildpackOrder = "release-generates-bad-yaml"

			cpBuildpack("release-generates-bad-yaml")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session).Should(gexec.Exit(224))
			Expect(session.Err).Should(gbytes.Say("buildpack's release output invalid"))
		})
	})

	Context("when the buildpack fails to release", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-release"

			cpBuildpack("fails-to-release")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session).Should(gexec.Exit(224))
			Expect(session.Err).Should(gbytes.Say("Failed to build droplet release"))
		})
	})

	Context("with a nested buildpack", func() {
		BeforeEach(func() {
			nestedBuildpack := "nested-buildpack"
			buildpackOrder = nestedBuildpack

			nestedBuildpackHash := "70d137ae4ee01fbe39058ccdebf48460"

			nestedBuildpackDir := filepath.Join(buildpacksDir, nestedBuildpackHash)
			err := os.MkdirAll(nestedBuildpackDir, 0777)
			Expect(err).NotTo(HaveOccurred())

			cp(filepath.Join(buildpackFixtures, "always-detects"), nestedBuildpackDir)
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should detect the nested buildpack", func() {
			Eventually(builder()).Should(gexec.Exit(0))
		})
	})
})

func removeTrailingSpace(dirty []string) []string {
	clean := []string{}
	for _, s := range dirty {
		clean = append(clean, strings.TrimRight(s, "\r\n"))
	}

	return clean
}
