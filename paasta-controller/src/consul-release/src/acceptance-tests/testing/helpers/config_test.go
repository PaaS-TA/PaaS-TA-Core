package helpers_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/consul-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func writeConfigJSON(json string) (string, error) {
	tempFile, err := ioutil.TempFile("", "config")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(tempFile.Name(), []byte(json), os.ModePerm)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

var _ = Describe("configuration", func() {
	Describe("LoadConfig", func() {
		Context("with a valid config JSON", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "https://some-bosh-target:25555",
						"username": "some-bosh-username",
						"password": "some-bosh-password",
						"director_ca_cert": "some-ca-cert"
					},
					"parallel_nodes": 4,
					"windows_clients": true
				}`)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("loads the config from the given path", func() {
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.Config{
					BOSH: helpers.ConfigBOSH{
						Target:         "https://some-bosh-target:25555",
						Host:           "some-bosh-target",
						Username:       "some-bosh-username",
						Password:       "some-bosh-password",
						DirectorCACert: "some-ca-cert",
					},
					ParallelNodes:  4,
					WindowsClients: true,
				}))
			})
		})

		Context("when parallel_nodes is missing", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "https://some-bosh-target:25555",
						"username": "some-bosh-username",
						"password": "some-bosh-password",
						"director_ca_cert": "some-ca-cert"
					}
				}`)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses parallel_nodes: 1", func() {
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.Config{
					BOSH: helpers.ConfigBOSH{
						Target:         "https://some-bosh-target:25555",
						Host:           "some-bosh-target",
						Username:       "some-bosh-username",
						Password:       "some-bosh-password",
						DirectorCACert: "some-ca-cert",
					},
					ParallelNodes: 1,
				}))
			})
		})

		Context("failure cases", func() {
			Context("with an missing config json file location", func() {
				It("should return an error", func() {
					_, err := helpers.LoadConfig("someblahblahfile")
					Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
				})
			})

			Context("when config file contains invalid JSON", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON("%%%")
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(ContainSubstring("invalid character '%'")))
				})
			})

			Context("when the bosh.target is missing", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON(`{}`)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(errors.New("missing `bosh.target` - e.g. 'https://192.168.50.4:25555'")))
				})
			})

			Context("when the bosh.target is invalid", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON(`{
						"bosh": {
							"target": "%%%"
						}
					}`)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(`parse %%%: invalid URL escape "%%%"`))
				})
			})

			Context("when the bosh.director_ca_cert is missing", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON(`{
						"bosh": {
							"target": "some-bosh-target"
						}
					}`)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(errors.New("missing `bosh.director_ca_cert` - specify CA cert for BOSH director validation")))
				})
			})

			Context("when the bosh.username is missing", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON(`{
						"bosh": {
							"target": "some-bosh-target",
							"director_ca_cert": "some-ca-cert"
						}
					}`)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(errors.New("missing `bosh.username` - specify username for authenticating with BOSH")))
				})
			})

			Context("when the bosh.password is missing", func() {
				var configFilePath string

				BeforeEach(func() {
					var err error
					configFilePath, err = writeConfigJSON(`{
						"bosh": {
							"target": "https://some-bosh-target:25555",
							"director_ca_cert": "some-ca-cert",
							"username": "some-bosh-username"
						}
					}`)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					err := os.Remove(configFilePath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return an error", func() {
					_, err := helpers.LoadConfig(configFilePath)
					Expect(err).To(MatchError(errors.New("missing `bosh.password` - specify password for authenticating with BOSH")))
				})
			})
		})
	})

	Describe("ConsulReleaseVersion", func() {
		var releaseVersion string

		BeforeEach(func() {
			releaseVersion = os.Getenv("CONSUL_RELEASE_VERSION")
		})

		AfterEach(func() {
			os.Setenv("CONSUL_RELEASE_VERSION", releaseVersion)
		})

		It("retrieves the consul release version number from the env", func() {
			os.Setenv("CONSUL_RELEASE_VERSION", "some-release-number")
			version := helpers.ConsulReleaseVersion()
			Expect(version).To(Equal("some-release-number"))
		})

		It("returns 'latest' if the env is not set", func() {
			os.Setenv("CONSUL_RELEASE_VERSION", "")
			version := helpers.ConsulReleaseVersion()
			Expect(version).To(Equal("latest"))
		})
	})

	Describe("ConfigPath", func() {
		var configPath string

		BeforeEach(func() {
			configPath = os.Getenv("CONSATS_CONFIG")
		})

		AfterEach(func() {
			os.Setenv("CONSATS_CONFIG", configPath)
		})

		Context("when a valid path is set", func() {
			It("returns the path", func() {
				os.Setenv("CONSATS_CONFIG", "/tmp/some-config.json")
				path, err := helpers.ConfigPath()
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal("/tmp/some-config.json"))
			})
		})

		Context("when path is not set", func() {
			It("returns an error", func() {
				os.Setenv("CONSATS_CONFIG", "")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(`$CONSATS_CONFIG "" does not specify an absolute path to test config file`))
			})
		})

		Context("when the path is not absolute", func() {
			It("returns an error", func() {
				os.Setenv("CONSATS_CONFIG", "some/path.json")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(`$CONSATS_CONFIG "some/path.json" does not specify an absolute path to test config file`))
			})
		})
	})
})
