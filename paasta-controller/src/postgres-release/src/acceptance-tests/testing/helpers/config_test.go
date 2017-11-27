package helpers_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func writeConfigFile(data string) (string, error) {
	tempFile, err := ioutil.TempFile("", "config")
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(tempFile.Name(), []byte(data), os.ModePerm)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

var _ = Describe("Configuration", func() {
	Describe("Load configuration", func() {
		Context("With a valid config file", func() {
			var configFilePath string

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Load the yaml content from the provided path without defaults", func() {
				var err error
				var data = `
postgres_release_version: "some-version"
postgresql_version: "some-version"
versions_file: "some-path"
bosh:
  target: some-target
  username: some-username
  password: some-password
  director_ca_cert: some-ca-cert
cloud_configs:
  default_azs: ["some-az1", "some-az2"]
  default_networks:
  - name: some-net1
  - name: some-net2
    static_ips:
    - some-ip1
    - some-ip2
    default: [some-default1, some-default2]
  default_persistent_disk_type: some-type
  default_vm_type: some-vm-type
`
				configFilePath, err = writeConfigFile(data)
				Expect(err).NotTo(HaveOccurred())
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(config).To(Equal(helpers.PgatsConfig{
					PGReleaseVersion:  "some-version",
					PostgreSQLVersion: "some-version",
					VersionsFile: "some-path",
					Bosh: helpers.BOSHConfig{
						Target:         "some-target",
						Username:       "some-username",
						Password:       "some-password",
						DirectorCACert: "some-ca-cert",
					},
					BoshCC: helpers.BOSHCloudConfig{
						AZs: []string{"some-az1", "some-az2"},
						Networks: []helpers.BOSHJobNetwork{
							helpers.BOSHJobNetwork{
								Name: "some-net1",
							},
							helpers.BOSHJobNetwork{
								Name:      "some-net2",
								StaticIPs: []string{"some-ip1", "some-ip2"},
								Default:   []string{"some-default1", "some-default2"},
							},
						},
						PersistentDiskType: "some-type",
						VmType:             "some-vm-type",
					},
				}))
			})

			It("Load the yaml content from the provided path with defaults", func() {
				var err error
				var data = `
bosh:
  director_ca_cert: some-ca-cert
`
				configFilePath, err = writeConfigFile(data)
				Expect(err).NotTo(HaveOccurred())
				config, err := helpers.LoadConfig(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				result := helpers.DefaultPgatsConfig
				result.Bosh.DirectorCACert = "some-ca-cert"
				Expect(config).To(Equal(result))
			})
		})

		Context("With an invalid config yaml location", func() {
			It("Should return an error that the file does not exist", func() {
				_, err := helpers.LoadConfig("notExistentPath")
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})

		Context("With an incorrect config yaml content", func() {
			var configFilePath string

			AfterEach(func() {
				err := os.Remove(configFilePath)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should return an error if not a valid yaml", func() {
				var err error
				configFilePath, err = writeConfigFile("%%%")
				Expect(err).NotTo(HaveOccurred())

				_, err = helpers.LoadConfig(configFilePath)
				Expect(err).To(MatchError(ContainSubstring("yaml: could not find expected directive name")))
			})

			It("Should return an error if BOSH CA Cert missing", func() {
				var err error
				configFilePath, err = writeConfigFile("---")
				Expect(err).NotTo(HaveOccurred())

				_, err = helpers.LoadConfig(configFilePath)
				Expect(err).To(MatchError(errors.New(helpers.MissingCertificateMsg)))
			})
		})
	})

	Describe("ConfigPath", func() {
		var configPath string

		BeforeEach(func() {
			configPath = os.Getenv("PGATS_CONFIG")
		})

		AfterEach(func() {
			os.Setenv("PGATS_CONFIG", configPath)
		})

		Context("when a valid path is set", func() {
			It("returns the path", func() {
				os.Setenv("PGATS_CONFIG", "/tmp/some-config.json")
				path, err := helpers.ConfigPath()
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal("/tmp/some-config.json"))
			})
		})

		Context("when path is not set", func() {
			It("returns an error", func() {
				os.Setenv("PGATS_CONFIG", "")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(fmt.Errorf(helpers.IncorrectEnvMsg, "")))
			})
		})

		Context("when the path is not absolute", func() {
			It("returns an error", func() {
				os.Setenv("PGATS_CONFIG", "some/path.json")
				_, err := helpers.ConfigPath()
				Expect(err).To(MatchError(fmt.Errorf(helpers.IncorrectEnvMsg, "some/path.json")))
			})
		})
	})
})
