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
		Context("with a valid config options", func() {
			var configFilePath string

			BeforeEach(func() {
				var err error
				configFilePath, err = writeConfigJSON(`{
					"bosh": {
						"target": "some-bosh-target",
						"username": "some-bosh-username",
						"password": "some-bosh-password",
						"director_ca_cert": "some-ca-cert"
					},
					"aws": {
						"subnets": [
						{"id":"some-awssubnet-1", "range": "10.0.1.0/24", "az":"some-az-1", "security_group":"some-security-group-1"},
						{"id":"some-awssubnet-2", "range": "10.0.2.0/24", "az":"some-az-2", "security_group":"some-security-group-2"}
						],
						"cloud_config_subnets": [
						{"id":"some-cloudconfig-awssubnet-1", "range": "10.0.3.0/24", "az":"some-az-1", "security_group":"some-security-group-1"},
						{"id":"some-cloudconfig-awssubnet-2", "range": "10.0.4.0/24", "az":"some-az-2", "security_group":"some-security-group-2"}
						],
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
						Target:         "some-bosh-target",
						Username:       "some-bosh-username",
						Password:       "some-bosh-password",
						DirectorCACert: "some-ca-cert",
					},
					AWS: helpers.ConfigAWS{
						Subnets: []helpers.ConfigSubnet{
							{ID: "some-awssubnet-1", Range: "10.0.1.0/24", AZ: "some-az-1", SecurityGroup: "some-security-group-1"},
							{ID: "some-awssubnet-2", Range: "10.0.2.0/24", AZ: "some-az-2", SecurityGroup: "some-security-group-2"},
						},
						CloudConfigSubnets: []helpers.ConfigSubnet{
							{ID: "some-cloudconfig-awssubnet-1", Range: "10.0.3.0/24", AZ: "some-az-1", SecurityGroup: "some-security-group-1"},
							{ID: "some-cloudconfig-awssubnet-2", Range: "10.0.4.0/24", AZ: "some-az-2", SecurityGroup: "some-security-group-2"},
						},
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
					ParallelNodes:         4,
					WindowsClients:        true,
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
						Region:         "us-west-2",
					},
					TurbulenceReleaseName: "turbulence",
					ParallelNodes:         1,
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
						Region:         "us-west-2",
					},
					TurbulenceReleaseName: "turbulence",
					ParallelNodes:         1,
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

			It("uses the region 'us-west-2'", func() {
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
						Region:         "us-west-2",
					},
					TurbulenceReleaseName: "turbulence",
					ParallelNodes:         1,
				}))
			})
		})

		Context("when parallel_nodes is missing", func() {
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

			It("uses parallel_nodes: 1", func() {
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
						Region:         "us-west-2",
					},
					TurbulenceReleaseName: "turbulence",
					ParallelNodes:         1,
				}))
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
