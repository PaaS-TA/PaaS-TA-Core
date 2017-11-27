package helpers_test

import (
	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	Describe("Load Global properties", func() {
		Context("With a valid input and default values", func() {
			var props helpers.ManifestProperties

			It("Corretly load all the data", func() {
				var err error
				var data = `
databases:
  port: 5524
  databases:
  - citext: true
    name: sandbox
`
				err = props.LoadJobProperties("postgres", []byte(data))
				Expect(err).NotTo(HaveOccurred())
				expected := helpers.Properties{
					Databases: helpers.PgProperties{
						Port: 5524,
						Databases: []helpers.PgDBProperties{
							{CITExt: true,
								Name: "sandbox"},
						},
						MaxConnections:        500,
						LogLinePrefix:         "%m: ",
						CollectStatementStats: false,
					},
				}
				Expect(props.GetJobProperties("postgres")).To(Equal([]helpers.Properties{expected}))
			})
		})
		Context("With a valid input and no default values", func() {
			var props helpers.ManifestProperties

			BeforeEach(func() {
				var err error
				var data = `
databases:
  port: 5524
  address: x.x.x.x
  databases:
  - citext: true
    name: sandbox
  - citext: true
    name: sandbox2
  roles:
  - name: pgadmin
    password: admin
  - name: pgadmin2
    password: admin
  max_connections: 10
  log_line_prefix: "%d"
  collect_statement_statistics: true
  monit_timeout: 120
  additional_config:
    max_wal_senders: 5
    archive_timeout: 1800s
`
				err = props.LoadJobProperties("postgres", []byte(data))
				Expect(err).NotTo(HaveOccurred())
			})

			It("Correctly load all the data", func() {
				m := make(helpers.PgAdditionalConfigMap)
				m["archive_timeout"] = "1800s"
				m["max_wal_senders"] = 5
				expected := helpers.Properties{
					Databases: helpers.PgProperties{
						Port: 5524,
						Databases: []helpers.PgDBProperties{
							{CITExt: true,
								Name: "sandbox"},
							{CITExt: true,
								Name: "sandbox2"},
						},
						Roles: []helpers.PgRoleProperties{
							{Password: "admin",
								Name: "pgadmin"},
							{Password: "admin",
								Name: "pgadmin2"},
						},
						MaxConnections:        10,
						LogLinePrefix:         "%d",
						CollectStatementStats: true,
						MonitTimeout:          120,
						AdditionalConfig:      m,
					},
				}
				Expect(props.GetJobProperties("postgres")).To(Equal([]helpers.Properties{expected}))

			})
		})
		Context("With a invalid input", func() {
			var props helpers.ManifestProperties

			It("Fail to load the an invalid yaml", func() {
				var err error
				err = props.LoadJobProperties("xxx", []byte("%%%"))
				Expect(err).To(MatchError(ContainSubstring("yaml: could not find expected directive name")))
			})
		})
	})
})
