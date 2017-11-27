package helpers_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
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
		Context("with a valid config options", func() {
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
					"aws": {
						"subnet": "some-awssubnet",
						"access_key_id": "some-access-key-id",
						"secret_access_key": "some-secret-access-key",
						"default_key_name": "some-default-key-name",
						"default_security_groups": ["some-default-security-group"],
						"region": "some-region"
					},
					"registry": {
						"host": "some-registry-host",
						"port": 12345,
						"username": "some-registry-username",
						"password": "some-registry-password"
					},
					"cf" : {
						"domain": "api.some.domain.com"
					}
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
					AWS: helpers.ConfigAWS{
						Subnet:                "some-awssubnet",
						AccessKeyID:           "some-access-key-id",
						SecretAccessKey:       "some-secret-access-key",
						DefaultKeyName:        "some-default-key-name",
						DefaultSecurityGroups: []string{"some-default-security-group"},
						Region:                "some-region",
					},
					Registry: helpers.ConfigRegistry{
						Host:     "some-registry-host",
						Port:     12345,
						Username: "some-registry-username",
						Password: "some-registry-password",
					},
					TurbulenceReleaseName: "turbulence",
					CF: helpers.ConfigCF{
						Domain: "api.some.domain.com",
					},
				}))
			})
		})

		Context("with an invalid config json file location", func() {
			It("should return an error if the file does not exist", func() {
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
				Expect(err).To(MatchError(errors.New("missing `bosh.target` - e.g. 'lite' or '192.168.50.4'")))
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

		Context("when the bosh.username is missing", func() {
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
				Expect(err).To(MatchError(errors.New("missing `bosh.username` - specify username for authenticating with BOSH")))
			})
		})

		Context("when the bosh_password is missing", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "some-bosh-target",
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

		Context("when turbulence config is not provided", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "some-bosh-target",
						"username": "some-bosh-username",
						"password": "some-bosh-password"
					}
				}`)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a valid config", func() {
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.Config{
					BOSH: helpers.ConfigBOSH{
						Target:   "some-bosh-target",
						Username: "some-bosh-username",
						Password: "some-bosh-password",
					},
					AWS: helpers.ConfigAWS{
						DefaultKeyName: "bosh",
						Region:         "us-east-1",
					},
					TurbulenceReleaseName: "turbulence",
				}))
			})
		})

		Context("when aws.default_key_name is missing", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "some-bosh-target",
						"username": "some-bosh-username",
						"password": "some-bosh-password"
					}
				}`)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the name 'bosh'", func() {
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.Config{
					BOSH: helpers.ConfigBOSH{
						Target:   "some-bosh-target",
						Username: "some-bosh-username",
						Password: "some-bosh-password",
					},
					AWS: helpers.ConfigAWS{
						DefaultKeyName: "bosh",
						Region:         "us-east-1",
					},
					TurbulenceReleaseName: "turbulence",
				}))
			})
		})

		Context("when aws.region is missing", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "some-bosh-target",
						"username": "some-bosh-username",
						"password": "some-bosh-password"
					}
				}`)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses the region 'us-east-1'", func() {
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.Config{
					BOSH: helpers.ConfigBOSH{
						Target:   "some-bosh-target",
						Username: "some-bosh-username",
						Password: "some-bosh-password",
					},
					AWS: helpers.ConfigAWS{
						DefaultKeyName: "bosh",
						Region:         "us-east-1",
					},
					TurbulenceReleaseName: "turbulence",
				}))
			})
		})
	})

	Describe("EtcdDevReleaseVersion", func() {
		var releaseVersion string

		BeforeEach(func() {
			releaseVersion = os.Getenv("ETCD_RELEASE_VERSION")
		})

		AfterEach(func() {
			os.Setenv("ETCD_RELEASE_VERSION", releaseVersion)
		})

		It("retrieves the consul release version number from the env", func() {
			os.Setenv("ETCD_RELEASE_VERSION", "some-release-number")
			version := helpers.EtcdDevReleaseVersion()
			Expect(version).To(Equal("some-release-number"))
		})

		It("returns 'latest' if the env is not set", func() {
			os.Setenv("ETCD_RELEASE_VERSION", "")
			version := helpers.EtcdDevReleaseVersion()
			Expect(version).To(Equal("latest"))
		})
	})

	Describe("ConfigPath", func() {
		var configPath string

		BeforeEach(func() {
			configPath = os.Getenv("EATS_CONFIG")
		})

		AfterEach(func() {
			os.Setenv("EATS_CONFIG", configPath)
		})

		Context("when a valid path is set", func() {
			It("returns the path", func() {
				os.Setenv("EATS_CONFIG", "/tmp/some-config.json")
				path, err := helpers.ConfigPath()
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal("/tmp/some-config.json"))
			})
		})

		Context("when path is not set", func() {
			It("returns an error", func() {
				os.Setenv("EATS_CONFIG", "")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(`$EATS_CONFIG "" does not specify an absolute path to test config file`))
			})
		})

		Context("when the path is not absolute", func() {
			It("returns an error", func() {
				os.Setenv("EATS_CONFIG", "some/path.json")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(`$EATS_CONFIG "some/path.json" does not specify an absolute path to test config file`))
			})
		})
	})
})
