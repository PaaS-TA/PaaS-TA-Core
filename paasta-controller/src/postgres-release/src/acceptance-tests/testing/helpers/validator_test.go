package helpers_test

import (
	"errors"
	"fmt"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate deployment", func() {
	var (
		validator helpers.Validator
		mocks     map[string]sqlmock.Sqlmock
	)
	BeforeEach(func() {
		manifestProps := helpers.Properties{
			Databases: helpers.PgProperties{
				Port: 5522,
				Databases: []helpers.PgDBProperties{
					helpers.PgDBProperties{
						CITExt: true,
						Name:   "db1",
					},
				},
				Roles: []helpers.PgRoleProperties{
					helpers.PgRoleProperties{
						Name:     "pgadmin",
						Password: "admin",
						Permissions: []string{
							"NOSUPERUSER",
							"CREATEDB",
							"CREATEROLE",
							"NOINHERIT",
							"REPLICATION",
							"CONNECTION LIMIT 20",
							"VALID UNTIL 'May 5 12:00:00 2017 +1'",
						},
					},
				},
				MaxConnections:        30,
				LogLinePrefix:         "xxx",
				CollectStatementStats: true,
				AdditionalConfig: helpers.PgAdditionalConfigMap{
					"max_wal_senders": 5,
					"archive_timeout": "1800s",
				},
			},
		}
		postgresData := helpers.PGOutputData{
			Roles: map[string]helpers.PGRole{
				"vcap": helpers.PGRole{
					Name: "vcap",
				},
				"pg_signal_backend": helpers.PGRole{
					Name: "pg_signal_backend",
				},
				"pgadmin": helpers.PGRole{
					Name:        "pgadmin",
					Super:       false,
					Inherit:     false,
					CreateRole:  true,
					CreateDb:    true,
					CanLogin:    true,
					Replication: true,
					ConnLimit:   20,
					ValidUntil:  "2017-05-05T11:00:00+00:00",
				},
			},
			Databases: []helpers.PGDatabase{
				helpers.PGDatabase{
					Name: helpers.DefaultDB,
					DBExts: []helpers.PGDatabaseExtensions{
						helpers.PGDatabaseExtensions{
							Name: "plpgsql",
						},
					},
				},
				helpers.PGDatabase{
					Name: "db1",
					DBExts: []helpers.PGDatabaseExtensions{
						helpers.PGDatabaseExtensions{
							Name: "pgcrypto",
						},
						helpers.PGDatabaseExtensions{
							Name: "plpgsql",
						},
						helpers.PGDatabaseExtensions{
							Name: "citext",
						},
						helpers.PGDatabaseExtensions{
							Name: "pg_stat_statements",
						},
					},
					Tables: []helpers.PGTable{},
				},
			},
			Settings: map[string]string{
				"log_line_prefix": "xxx",
				"max_wal_senders": "5",
				"archive_timeout": "1800s",
				"port":            "5522",
				"other":           "other",
				"max_connections": "30",
			},
			Version: helpers.PGVersion{Version: "PostgreSQL 9.4.9"},
		}

		mocks = make(map[string]sqlmock.Sqlmock)
		db, mock, err := sqlmock.New()
		Expect(err).NotTo(HaveOccurred())
		mocks[helpers.DefaultDB] = mock
		pg := helpers.PGData{
			Data: helpers.PGCommon{},
			DBs: []helpers.PGConn{
				helpers.PGConn{
					DB:       db,
					TargetDB: helpers.DefaultDB,
				},
			},
		}

		validator = helpers.Validator{
			ManifestProps:     manifestProps,
			PostgresData:      postgresData,
			PG:                pg,
			PostgreSQLVersion: "PostgreSQL 9.4.9",
		}
	})

	Describe("Validate a good deployment", func() {
		Context("Validate all", func() {
			It("Properly validates dbs", func() {
				err := validator.ValidateDatabases()
				Expect(err).NotTo(HaveOccurred())
			})
			It("Properly validates roles", func() {
				input := "May 5 12:00:00 2017 +1"
				expected := "2017-05-05T11:00:00+00:00"
				err := mockDate(input, expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				err = validator.ValidateRoles()
				Expect(err).NotTo(HaveOccurred())
			})
			It("Properly validates settings", func() {
				err := validator.ValidateSettings()
				Expect(err).NotTo(HaveOccurred())
			})
			It("Properly validates PostgreSQL version", func() {
				err := validator.ValidatePostgreSQLVersion()
				Expect(err).NotTo(HaveOccurred())
			})
			It("Properly validates all", func() {
				input := "May 5 12:00:00 2017 +1"
				expected := "2017-05-05T11:00:00+00:00"
				err := mockDate(input, expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				err = validator.ValidateAll()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Describe("Validate a bad deployment", func() {
		Context("Validate databases", func() {
			It("Fails if DB missing", func() {
				validator.ManifestProps.Databases.Databases = []helpers.PgDBProperties{
					helpers.PgDBProperties{
						CITExt: true,
						Name:   "db1",
					},
					helpers.PgDBProperties{
						CITExt: true,
						Name:   "zz2",
					},
				}
				err := validator.ValidateDatabases()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.MissingDatabaseValidationError, "zz2"))))
			})
			It("Fails if DB extension missing", func() {
				validator.PostgresData.Databases[1].DBExts = []helpers.PGDatabaseExtensions{}
				err := validator.ValidateDatabases()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.MissingExtensionValidationError, "pgcrypto", "db1"))))
			})
			It("Fails if extra database present", func() {
				validator.ManifestProps.Databases.Databases = []helpers.PgDBProperties{}
				err := validator.ValidateDatabases()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.ExtraDatabaseValidationError, "db1"))))
			})
			It("Fails if extra extension present", func() {
				validator.ManifestProps.Databases.Databases[0].CITExt = false
				err := validator.ValidateDatabases()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.ExtraExtensionValidationError, "citext", "db1"))))
			})
		})
		Context("Validate PostgreSQL version", func() {
			It("Fails if wrong PostgreSQL version", func() {
				validator.PostgreSQLVersion = "wrong value"
				err := validator.ValidatePostgreSQLVersion()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.WrongPostreSQLVersionError, "PostgreSQL 9.4.9", "wrong value"))))
			})
		})
		Context("Validate roles", func() {
			It("Fails if role missing", func() {
				validator.ManifestProps.Databases.Roles = []helpers.PgRoleProperties{
					helpers.PgRoleProperties{
						Name:     "pgadmin2",
						Password: "admin2",
					},
				}
				err := validator.ValidateRoles()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.MissingRoleValidationError, "pgadmin2"))))
			})
			It("Fails if incorrect role permission", func() {
				validator.ManifestProps.Databases.Roles = []helpers.PgRoleProperties{
					helpers.PgRoleProperties{
						Name:     "pgadmin",
						Password: "admin",
						Permissions: []string{
							"SUPERUSER",
							"CREATEDB",
							"CREATEROLE",
							"NOINHERIT",
							"REPLICATION",
							"CONNECTION LIMIT 21",
							"VALID UNTIL 'May 5 12:00:00 2017 +1'",
						},
					},
				}
				input := "May 5 12:00:00 2017 +1"
				expected := "2017-05-05T11:00:00+00:00"
				err := mockDate(input, expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				err = validator.ValidateRoles()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.IncorrectRolePrmissionValidationError, "pgadmin"))))
			})
		})
		Context("Validate settings", func() {
			It("Fails if additional prop value is incorrect", func() {
				validator.ManifestProps.Databases.AdditionalConfig["max_wal_senders"] = 10
				err := validator.ValidateSettings()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.IncorrectSettingValidationError, 10, 5, "max_wal_senders"))))
			})
			It("Fails if additional prop value is missing", func() {
				validator.ManifestProps.Databases.AdditionalConfig["some_prop"] = 10
				err := validator.ValidateSettings()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.MissingSettingValidationError, "some_prop"))))
			})
			It("Fails if port is wrong", func() {
				validator.ManifestProps.Databases.Port = 1111
				err := validator.ValidateSettings()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.IncorrectSettingValidationError, 1111, 5522, "port"))))
			})
			It("Fails if max connextions setting is wrong", func() {
				validator.ManifestProps.Databases.MaxConnections = 10
				err := validator.ValidateSettings()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.IncorrectSettingValidationError, 10, 30, "max_connections"))))
			})
			It("Fails if log_line_prefix setting is wrong", func() {
				validator.ManifestProps.Databases.LogLinePrefix = "yyy"
				err := validator.ValidateSettings()
				Expect(err).To(MatchError(errors.New(fmt.Sprintf(helpers.IncorrectSettingValidationError, "yyy", "xxx", "log_line_prefix"))))
			})
		})
	})
	Describe("Check consistency after an upgrade", func() {
		Context("Validate tables", func() {
			var dataBefore helpers.PGOutputData

			BeforeEach(func() {
				dataBefore = helpers.PGOutputData{
					Roles: map[string]helpers.PGRole{},
					Databases: []helpers.PGDatabase{
						helpers.PGDatabase{
							Name:   helpers.DefaultDB,
							DBExts: []helpers.PGDatabaseExtensions{},
							Tables: []helpers.PGTable{},
						},
						helpers.PGDatabase{
							Name:   "db1",
							DBExts: []helpers.PGDatabaseExtensions{},
							Tables: []helpers.PGTable{
								helpers.PGTable{
									SchemaName: "myschema1",
									TableName:  "mytable1",
									TableOwner: "myowner1",
									TableColumns: []helpers.PGTableColumn{
										helpers.PGTableColumn{
											ColumnName: "column1",
											DataType:   "type1",
											Position:   1,
										},
										helpers.PGTableColumn{
											ColumnName: "column2",
											DataType:   "type2",
											Position:   2,
										},
									},
									TableRowsCount: helpers.PGCount{Num: 90},
								},
								helpers.PGTable{
									SchemaName:     "myschema2",
									TableName:      "mytable2",
									TableOwner:     "myowner2",
									TableColumns:   []helpers.PGTableColumn{},
									TableRowsCount: helpers.PGCount{Num: 0},
								},
							},
						},
					},
					Settings: map[string]string{},
				}
				validator.PostgresData = dataBefore
			})
			It("Reports tables equals", func() {
				dataAfter, err := dataBefore.CopyData()
				Expect(err).NotTo(HaveOccurred())
				Expect(validator.CompareTablesTo(dataAfter)).To(BeTrue())
			})
			It("Reports tables equal if tables order differs", func() {
				dataAfter, err := dataBefore.CopyData()
				Expect(err).NotTo(HaveOccurred())
				dataAfter.Databases[1].Tables[0] = dataBefore.Databases[1].Tables[1]
				dataAfter.Databases[1].Tables[1] = dataBefore.Databases[1].Tables[0]

				Expect(validator.CompareTablesTo(dataAfter)).To(BeTrue())
			})
			It("Reports tables equal if tables columns order differs", func() {
				dataAfter, err := dataBefore.CopyData()
				Expect(err).NotTo(HaveOccurred())
				dataAfter.Databases[1].Tables[0].TableColumns[0] = dataBefore.Databases[1].Tables[0].TableColumns[1]
				dataAfter.Databases[1].Tables[0].TableColumns[1] = dataBefore.Databases[1].Tables[0].TableColumns[0]

				Expect(validator.CompareTablesTo(dataAfter)).To(BeTrue())
			})
			It("Reports table different if table missing", func() {
				dataAfter, err := dataBefore.CopyData()
				Expect(err).NotTo(HaveOccurred())
				dataAfter.Databases[1].Tables = []helpers.PGTable{
					helpers.PGTable{
						SchemaName: "myschema2",
						TableName:  "mytable2",
						TableOwner: "myowner2",
						TableColumns: []helpers.PGTableColumn{
							helpers.PGTableColumn{
								ColumnName: "column1",
								DataType:   "type1",
								Position:   1,
							},
						},
						TableRowsCount: helpers.PGCount{Num: 0},
					},
				}
				Expect(validator.CompareTablesTo(dataAfter)).To(BeFalse())
			})

		})
	})
})
