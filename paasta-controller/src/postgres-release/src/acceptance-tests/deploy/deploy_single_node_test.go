package deploy_test

import (
	"os"
	"strconv"

	cfgtypes "github.com/cloudfoundry/config-server/types"
	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func createOrUpdateDeployment(postgresReleaseVersion int, manifestPath string, name string, variables map[string]interface{}) error {
	var err error
	var vars map[string]interface{}
	if variables != nil {
		vars = variables
	} else {
		vars = make(map[string]interface{})
	}
	releases := make(map[string]string)
	if postgresReleaseVersion != -1 {
		releases["postgres"] = strconv.Itoa(postgresReleaseVersion)
		err = director.UploadPostgresReleaseFromURL(postgresReleaseVersion)
		if err != nil {
			return err
		}
	}
	err = director.SetDeploymentFromManifest(manifestPath, releases, name)
	if err != nil {
		return err
	}
	if director.GetEnv(name).ContainsVariables() || variables != nil {
		if director.GetEnv(name).ContainsVariables() {
			if _, err = director.GetEnv(name).GetVmAddress("postgres"); err != nil {

				vars["postgres_host"] = "1.1.1.1"
				err = director.GetEnv(name).EvaluateTemplate(vars, helpers.EvaluateOptions{})
				if err != nil {
					return err
				}
				err = director.GetEnv(name).CreateOrUpdateDeployment()
				if err != nil {
					return err
				}
			}
			var pgHost string
			pgHost, err = director.GetEnv(name).GetVmDNS("postgres")
			if err != nil {
				pgHost, err = director.GetEnv(name).GetVmAddress("postgres")
				if err != nil {
					return err
				}
			}
			vars["postgres_host"] = pgHost

			err = director.SetDeploymentFromManifest(manifestPath, releases, name)
			if err != nil {
				return err
			}
		}
		err = director.GetEnv(name).EvaluateTemplate(vars, helpers.EvaluateOptions{})
		if err != nil {
			return err
		}
	}
	err = director.GetEnv(name).CreateOrUpdateDeployment()
	if err != nil {
		return err
	}
	return nil
}
func getPostgresJobProps(envName string) (helpers.Properties, error) {
	var err error
	manifestProps, err := director.GetEnv(envName).GetJobsProperties()
	if err != nil {
		return helpers.Properties{}, err
	}
	pgprops := manifestProps.GetJobProperties("postgres")[0]
	return pgprops, nil
}

func connectToPostgres(envName string, variables map[string]interface{}) (helpers.Properties, helpers.PGData, error) {

	pgprops, err := getPostgresJobProps(envName)
	if err != nil {
		return helpers.Properties{}, helpers.PGData{}, err
	}
	var pgHost string
	pgHost, err = director.GetEnv(envName).GetVmDNS("postgres")
	if err != nil {
		pgHost, err = director.GetEnv(envName).GetVmAddress("postgres")
		if err != nil {
			return helpers.Properties{}, helpers.PGData{}, err
		}
	}

	pgc := helpers.PGCommon{
		Address: pgHost,
		Port:    pgprops.Databases.Port,
		DefUser: helpers.User{
			Name:     variables["defuser_name"].(string),
			Password: variables["defuser_password"].(string),
		},
		AdminUser: helpers.User{
			Name:     variables["superuser_name"].(string),
			Password: variables["superuser_password"].(string),
		},
		CertUser: helpers.User{},
	}
	DB, err := helpers.NewPostgres(pgc)
	if err != nil {
		return helpers.Properties{}, helpers.PGData{}, err
	}
	return pgprops, DB, nil
}

var _ = Describe("Deploy single instance", func() {

	var envName string
	var DB helpers.PGData
	var pgprops helpers.Properties
	var manifestPath, deploymentPrefix string
	var version int
	var latestPostgreSQLVersion string
	var variables map[string]interface{}

	JustBeforeEach(func() {
		var err error
		envName = helpers.GenerateEnvName(deploymentPrefix)
		latestPostgreSQLVersion = configParams.PostgreSQLVersion
		if latestPostgreSQLVersion == "current" {
			latestPostgreSQLVersion = versions.GetPostgreSQLVersion(versions.GetLatestVersion())
		}

		variables["defuser_name"] = "pgadmin"
		variables["defuser_password"] = "admin"
		variables["superuser_name"] = "superuser"
		variables["superuser_password"] = "superpsw"

		By("Deploying a single postgres instance")
		err = createOrUpdateDeployment(version, manifestPath, envName, variables)
		Expect(err).NotTo(HaveOccurred())

		By("Initializing a postgres client connection")
		pgprops, DB, err = connectToPostgres(envName, variables)
		Expect(err).NotTo(HaveOccurred())
		By("Populating the database")
		err = DB.CreateAndPopulateTables(pgprops.Databases.Databases[0].Name, helpers.SmallLoad)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("With a fresh deployment", func() {

		BeforeEach(func() {
			manifestPath = "../testing/templates/postgres_simple.yml"
			version = -1
			deploymentPrefix = "fresh"
			variables = make(map[string]interface{})

			variables["certs_matching_certs"] = "certuser_matching_certs"
			variables["certs_matching_name"] = "certuser_matching_name"

			variables["certs_mapped_certs"] = "certuser_mapped_certs"
			variables["certs_mapped_name"] = "certuser_mapped_name"
			variables["certs_mapped_cn"] = "certuser mapped cn"

			variables["certs_wrong_certs"] = "certuser_wrong_certs"
			variables["certs_wrong_cn"] = "certuser_wrong_cn"

			variables["certs_bad_ca"] = "bad_ca"
		})

		It("Successfully deploys a fresh env", func() {
			pgData, err := DB.GetData()
			Expect(err).NotTo(HaveOccurred())
			validator := helpers.NewValidator(pgprops, pgData, DB, latestPostgreSQLVersion)
			err = validator.ValidateAll()
			Expect(err).NotTo(HaveOccurred())

			By("Enabling SSL")
			manifestPath = "../testing/templates/postgres_simple_ssl.yml"
			err = createOrUpdateDeployment(version, manifestPath, envName, variables)
			Expect(err).NotTo(HaveOccurred())
			By("Re-initializing a postgres client connection")
			DB.CloseConnections()
			pgprops, DB, err = connectToPostgres(envName, variables)
			Expect(err).NotTo(HaveOccurred())

			pgprops, err := getPostgresJobProps(envName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pgprops.Databases.TLS).NotTo(Equal(helpers.PgTLS{}))
			rootCertPath, err := helpers.WriteFile(pgprops.Databases.TLS.CA)
			Expect(err).NotTo(HaveOccurred())
			badCAcerts := director.GetEnv(envName).GetVariable(variables["certs_bad_ca"].(string))
			badCaPath, err := helpers.WriteFile(badCAcerts.(cfgtypes.CertResponse).Certificate)
			Expect(err).NotTo(HaveOccurred())

			By("Checking non-secure connections")
			_, err = DB.GetPostgreSQLVersion()
			if err != nil {
				Expect(err.Error()).NotTo(HaveOccurred())
			}

			By("Checking secure connections")
			err = DB.ChangeSSLMode("verify-full", badCaPath)
			Expect(err).NotTo(HaveOccurred())
			_, err = DB.GetPostgreSQLVersion()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("x509"))

			err = os.Remove(badCaPath)
			Expect(err).NotTo(HaveOccurred())

			err = DB.ChangeSSLMode("verify-ca", rootCertPath)
			Expect(err).NotTo(HaveOccurred())
			_, err = DB.GetPostgreSQLVersion()
			if err != nil {
				Expect(err.Error()).NotTo(HaveOccurred())
			}

			err = DB.ChangeSSLMode("verify-full", rootCertPath)
			Expect(err).NotTo(HaveOccurred())
			_, err = DB.GetPostgreSQLVersion()
			if err != nil {
				Expect(err.Error()).NotTo(HaveOccurred())
			}

			By("Using cert authentication for client connection")
			certs := director.GetEnv(envName).GetVariable(variables["certs_matching_certs"].(string))
			err = DB.SetCertUserCertificates(variables["certs_matching_name"].(string), certs)
			Expect(err).NotTo(HaveOccurred())
			err = DB.UseCertAuthentication(true)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				_, err = DB.GetPostgreSQLVersion()
				if err != nil {
					return err.Error()
				}
				return ""
			}, "30s", "5s").Should(BeEmpty())

			certs = director.GetEnv(envName).GetVariable(variables["certs_wrong_certs"].(string))
			err = DB.SetCertUserCertificates(DB.Data.CertUser.Name, certs)
			Expect(err).NotTo(HaveOccurred())
			_, err = DB.GetPostgreSQLVersion()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("certificate authentication failed"))

			certs = director.GetEnv(envName).GetVariable(variables["certs_mapped_certs"].(string))
			err = DB.SetCertUserCertificates(variables["certs_mapped_name"].(string), certs)
			Expect(err).NotTo(HaveOccurred())
			_, err = DB.GetPostgreSQLVersion()
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Describe("Upgrading an existent env", func() {

		AssertUpgradeSuccessful := func() func() {
			return func() {
				var err error
				By("Validating the database has been deployed as requested")
				pgData, err := DB.GetData()
				Expect(err).NotTo(HaveOccurred())
				validator := helpers.NewValidator(pgprops, pgData, DB, versions.GetPostgreSQLVersion(version))
				err = validator.ValidateAll()
				Expect(err).NotTo(HaveOccurred())

				By("Upgrading to the new release")
				err = createOrUpdateDeployment(-1, manifestPath, director.GetEnv(envName).Deployment.Name(), variables)
				Expect(err).NotTo(HaveOccurred())

				By("Validating the database content is still valid after upgrade")
				pgDataAfter, err := DB.GetData()
				Expect(err).NotTo(HaveOccurred())

				tablesEqual := validator.CompareTablesTo(pgDataAfter)
				Expect(tablesEqual).To(BeTrue())

				By("Validating the database has been upgraded as requested")
				validator = helpers.NewValidator(pgprops, pgDataAfter, DB, latestPostgreSQLVersion)
				err = validator.ValidateAll()
				Expect(err).NotTo(HaveOccurred())

				By("Validating the VM can still be restarted")
				err = director.GetEnv(envName).Restart("postgres")
				Expect(err).NotTo(HaveOccurred())

			}
		}

		Context("Upgrading from older version", func() {
			BeforeEach(func() {
				manifestPath = "../testing/templates/postgres_simple_nolinks.yml"
				version = versions.GetOlderVersion()
				deploymentPrefix = "upg-older"
				variables = make(map[string]interface{})
			})
			It("Successfully upgrades from older", AssertUpgradeSuccessful())
		})
		Context("Upgrading from old version", func() {
			BeforeEach(func() {
				manifestPath = "../testing/templates/postgres_simple_nolinks.yml"
				version = versions.GetOldVersion()
				deploymentPrefix = "upg-old"
				variables = make(map[string]interface{})
			})
			It("Successfully upgrades from old", AssertUpgradeSuccessful())
		})
		Context("Upgrading from master version", func() {
			BeforeEach(func() {
				manifestPath = "../testing/templates/postgres_simple_nolinks.yml"
				version = versions.GetLatestVersion()
				deploymentPrefix = "upg-master"
				variables = make(map[string]interface{})
			})
			It("Successfully upgrades from master", AssertUpgradeSuccessful())
		})
	})

	AfterEach(func() {
		var err error
		if DB.Data.SSLRootCert != "" {
			err = os.Remove(DB.Data.SSLRootCert)
			Expect(err).NotTo(HaveOccurred())
		}
		if DB.Data.CertUser.Certificate != "" {
			err = os.Remove(DB.Data.CertUser.Certificate)
			Expect(err).NotTo(HaveOccurred())
		}
		if DB.Data.CertUser.Key != "" {
			err = os.Remove(DB.Data.CertUser.Key)
			Expect(err).NotTo(HaveOccurred())
		}
		err = director.GetEnv(envName).DeleteDeployment()
		Expect(err).NotTo(HaveOccurred())
	})

})
