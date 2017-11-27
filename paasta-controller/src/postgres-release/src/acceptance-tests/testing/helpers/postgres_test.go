package helpers_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudfoundry/postgres-release/src/acceptance-tests/testing/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var expectedcolumns = []string{"row_to_json"}
var genericError = fmt.Errorf("some error")

func convertQuery(query string) string {
	//return helpers.GetFormattedQuery(query)
	result := strings.Replace(helpers.GetFormattedQuery(query), ")", "\\)", -1)
	result = strings.Replace(result, "(", "\\(", -1)
	result = strings.Replace(result, ":", "\\:", -1)
	result = strings.Replace(result, "+", "\\+", -1)
	result = strings.Replace(result, "'", "\\'", -1)
	return strings.Replace(result, "*", "(.+)", -1)
}

func mockSettings(expected map[string]string, mocks map[string]sqlmock.Sqlmock) {
	if expected == nil {
		mocks[helpers.DefaultDB].ExpectQuery(convertQuery(helpers.GetSettingsQuery)).WillReturnError(genericError)
	} else {
		rows := sqlmock.NewRows(expectedcolumns)
		for key, value := range expected {
			ff := "{\"name\": \"%s\", \"setting\": \"%s\", \"some1\": \"%s\",\"vartype\": \"%s\"}"
			row := fmt.Sprintf(ff, key, value, "some0", "string")
			rows = rows.AddRow(row)
		}
		mocks[helpers.DefaultDB].ExpectQuery(convertQuery(helpers.GetSettingsQuery)).WillReturnRows(rows)
	}
}

func mockDatabases(expected []helpers.PGDatabase, mocks map[string]sqlmock.Sqlmock) {
	if expected == nil {
		mocks["dbsuper"].ExpectQuery(convertQuery(helpers.ListDatabasesQuery)).WillReturnError(genericError)
	} else {
		rows := sqlmock.NewRows(expectedcolumns)
		for _, elem := range expected {
			ff := "{\"datname\": \"%s\"}"
			row := fmt.Sprintf(ff, elem.Name)
			rows = rows.AddRow(row)
			extrows := sqlmock.NewRows(expectedcolumns)
			for _, elem := range elem.DBExts {
				xx := "{\"extname\": \"%s\"}"
				extrow := fmt.Sprintf(xx, elem.Name)
				extrows = extrows.AddRow(extrow)
			}
			mocks[elem.Name+"super"].ExpectQuery(convertQuery(helpers.ListDBExtensionsQuery)).WillReturnRows(extrows)
			tableRows := sqlmock.NewRows(expectedcolumns)
			for _, tElem := range elem.Tables {
				xx := "{\"schemaname\": \"%s\", \"tablename\":\"%s\",\"tableowner\":\"%s\"}"
				tableRow := fmt.Sprintf(xx, tElem.SchemaName, tElem.TableName, tElem.TableOwner)
				tableRows = tableRows.AddRow(tableRow)
			}
			mocks[elem.Name+"super"].ExpectQuery(convertQuery(helpers.ListTablesQuery)).WillReturnRows(tableRows)
			for _, tElem := range elem.Tables {
				columnRows := sqlmock.NewRows(expectedcolumns)
				for _, col := range tElem.TableColumns {
					xx := "{\"column_name\": \"%s\", \"data_type\":\"%s\",\"ordinal_position\":%d}"
					columnRow := fmt.Sprintf(xx, col.ColumnName, col.DataType, col.Position)
					columnRows = columnRows.AddRow(columnRow)
				}
				mocks[elem.Name+"super"].ExpectQuery(convertQuery(fmt.Sprintf(helpers.ListTableColumnsQuery, tElem.SchemaName, tElem.TableName))).WillReturnRows(columnRows)
				countRows := sqlmock.NewRows(expectedcolumns)
				countRows = countRows.AddRow(fmt.Sprintf("{\"count\": %d}", tElem.TableRowsCount.Num))
				mocks[elem.Name+"super"].ExpectQuery(convertQuery(fmt.Sprintf(helpers.CountTableRowsQuery, tElem.TableName))).WillReturnRows(countRows)
			}
		}
		mocks["dbsuper"].ExpectQuery(convertQuery(helpers.ListDatabasesQuery)).WillReturnRows(rows)
	}
}
func mockRoles(expected map[string]helpers.PGRole, mocks map[string]sqlmock.Sqlmock) error {
	if expected == nil {
		mocks[helpers.DefaultDB].ExpectQuery(convertQuery(helpers.ListRolesQuery)).WillReturnError(genericError)
	} else {
		rows := sqlmock.NewRows(expectedcolumns)
		for _, elem := range expected {
			row, err := json.Marshal(elem)
			if err != nil {
				return err
			}
			rows = rows.AddRow(row)
		}
		mocks[helpers.DefaultDB].ExpectQuery(convertQuery(helpers.ListRolesQuery)).WillReturnRows(rows)
	}
	return nil
}
func mockDate(current string, expected string, mocks map[string]sqlmock.Sqlmock) error {
	sqlCommand := convertQuery(fmt.Sprintf(helpers.ConvertToDateCommand, current))
	if expected == "" {
		mocks[helpers.DefaultDB].ExpectQuery(sqlCommand).WillReturnError(genericError)
	} else {
		row := fmt.Sprintf("{\"timestamptz\": \"%s\"}", expected)
		rows := sqlmock.NewRows(expectedcolumns).AddRow(row)
		mocks[helpers.DefaultDB].ExpectQuery(sqlCommand).WillReturnRows(rows)
	}
	return nil
}
func mockPostgreSQLVersion(expected helpers.PGVersion, mocks map[string]sqlmock.Sqlmock) error {
	sqlCommand := convertQuery(helpers.GetPostgreSQLVersionQuery)
	if expected.Version == "" {
		mocks[helpers.DefaultDB].ExpectQuery(sqlCommand).WillReturnError(genericError)
	} else {
		row := fmt.Sprintf("{\"version\": \"%s\"}", expected.Version)
		rows := sqlmock.NewRows(expectedcolumns).AddRow(row)
		mocks[helpers.DefaultDB].ExpectQuery(sqlCommand).WillReturnRows(rows)
	}
	return nil
}

var _ = Describe("Postgres", func() {
	Describe("Copy output data", func() {
		Context("Validate that data is copied", func() {
			var from helpers.PGOutputData

			BeforeEach(func() {

				from = helpers.PGOutputData{
					Roles: map[string]helpers.PGRole{
						"role1": helpers.PGRole{
							Name: "role1",
						},
					},
					Databases: []helpers.PGDatabase{
						helpers.PGDatabase{
							Name: "db1",
							DBExts: []helpers.PGDatabaseExtensions{
								helpers.PGDatabaseExtensions{
									Name: "exta",
								},
							},
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
									},
									TableRowsCount: helpers.PGCount{Num: 90},
								},
							},
						},
					},
					Settings: map[string]string{
						"max_connections": "30",
					},
				}
			})
			It("Correctly copies roles", func() {
				to, err := from.CopyData()
				Expect(err).NotTo(HaveOccurred())
				Expect(to).To(Equal(from))
				r := to.Roles["role1"]
				r.Name = "role2"
				to.Roles["role1"] = r
				Expect(to).NotTo(Equal(from))
			})
			It("Correctly copies settings", func() {
				to, err := from.CopyData()
				Expect(err).NotTo(HaveOccurred())
				Expect(to).To(Equal(from))
				to.Settings["max_connections"] = "xxx"
				Expect(to).NotTo(Equal(from))
			})
			It("Correctly copies tables", func() {
				to, err := from.CopyData()
				Expect(err).NotTo(HaveOccurred())
				Expect(to).To(Equal(from))
				to.Databases[0].Tables[0].SchemaName = "xxxx"
				Expect(to).NotTo(Equal(from))
			})
			It("Correctly copies columns", func() {
				to, err := from.CopyData()
				Expect(err).NotTo(HaveOccurred())
				Expect(to).To(Equal(from))
				to.Databases[0].Tables[0].TableColumns[0].ColumnName = "xxxx"
				Expect(to).NotTo(Equal(from))
			})
		})
	})
	Describe("Validate common data", func() {
		Context("Fail if common data is invalid", func() {
			It("Fail if no address provided", func() {
				props := helpers.PGCommon{
					Port: 10,
					DefUser: helpers.User{
						Name:     "uu",
						Password: "pp",
					},
				}
				_, err := helpers.NewPostgres(props)
				Expect(err).To(MatchError(errors.New(helpers.MissingDBAddressErr)))
			})
			It("Fail if no port provided", func() {
				props := helpers.PGCommon{
					Address: "bb",
					DefUser: helpers.User{
						Name:     "uu",
						Password: "pp",
					},
				}
				_, err := helpers.NewPostgres(props)
				Expect(err).To(MatchError(errors.New(helpers.MissingDBPortErr)))
			})
			It("Fail if no default user provided", func() {
				props := helpers.PGCommon{
					Address: "bb",
					Port:    10,
					DefUser: helpers.User{
						Password: "pp",
					},
				}
				_, err := helpers.NewPostgres(props)
				Expect(err).To(MatchError(errors.New(helpers.MissingDefaultUserErr)))
			})
			It("Fail if no default password provided", func() {
				props := helpers.PGCommon{
					Address: "bb",
					Port:    10,
					DefUser: helpers.User{
						Name: "uu",
					},
				}
				_, err := helpers.NewPostgres(props)
				Expect(err).To(MatchError(errors.New(helpers.MissingDefaultPasswordErr)))
			})
			It("Fail if incorrect data provided", func() {
				props := helpers.PGCommon{
					Address: "bb",
					Port:    10,
					DefUser: helpers.User{
						Name:     "uu",
						Password: "pp",
					},
				}
				pg, err := helpers.NewPostgres(props)
				Expect(err).NotTo(HaveOccurred())
				_, err = pg.GetDefaultConnection()
				Expect(err).To(MatchError(ContainSubstring("no such host")))
			})
			It("Fail if getting super user connection and no super user provided", func() {
				props := helpers.PGCommon{
					Address: "bb",
					Port:    10,
					DefUser: helpers.User{
						Name:     "uu",
						Password: "pp",
					},
				}
				pg, err := helpers.NewPostgres(props)
				Expect(err).NotTo(HaveOccurred())
				_, err = pg.GetSuperUserConnection()
				Expect(err).To(MatchError(helpers.NoSuperUserProvidedErr))
			})
			It("Fail if incorrect sslmode provided", func() {
				props := helpers.PGCommon{
					Address: "xx",
					SSLMode: "unknown",
					Port:    10,
					DefUser: helpers.User{
						Name:     "uu",
						Password: "pp",
					},
				}
				_, err := helpers.NewPostgres(props)
				Expect(err).To(MatchError(errors.New(helpers.IncorrectSSLModeErr)))
			})
		})
	})
	Describe("Validate SSL mode", func() {
		var (
			mocks map[string]sqlmock.Sqlmock
			pg    *helpers.PGData
		)
		BeforeEach(func() {
			mocks = make(map[string]sqlmock.Sqlmock)
			db, mock, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks[helpers.DefaultDB] = mock
			pg = &helpers.PGData{
				Data: helpers.PGCommon{
					SSLMode: "disable",
				},
				DBs: []helpers.PGConn{
					helpers.PGConn{
						DB:       db,
						TargetDB: helpers.DefaultDB,
					},
				},
			}
		})
		AfterEach(func() {
			pg.CloseConnections()
		})
		Context("Changing SSL mode", func() {
			It("Fails to change to an invalid ssl mode", func() {
				err := pg.ChangeSSLMode("unknown", "")
				Expect(err).To(MatchError(errors.New(helpers.IncorrectSSLModeErr)))
			})
			It("Correctly change to a valid ssl mode", func() {
				err := pg.ChangeSSLMode("require", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(pg.Data.SSLMode).To(Equal("require"))
			})
			It("Fails to change to stronger SSL modes if missing root cert", func() {
				err := pg.ChangeSSLMode("verify-ca", "")
				Expect(err).To(MatchError(errors.New(helpers.MissingSSLRootCertErr)))
			})
			It("Correctly change to SSL modes that require root cert", func() {
				err := pg.ChangeSSLMode("verify-ca", "/somepath")
				Expect(err).NotTo(HaveOccurred())
				Expect(pg.Data.SSLMode).To(Equal("verify-ca"))
			})
			It("Correctly close connections when changing ssl mode", func() {
				_, err := pg.GetDefaultConnection()
				Expect(err).NotTo(HaveOccurred())
				Expect(pg.DBs).NotTo(BeNil())
				err = pg.ChangeSSLMode("verify-full", "/some-path")
				Expect(err).NotTo(HaveOccurred())
				Expect(pg.Data.SSLMode).To(Equal("verify-full"))
				conns := 0
				if pg.DBs != nil {
					conns = len(pg.DBs)
				}
				Expect(conns).To(BeZero())
			})
		})
	})
	Describe("Run read-only queries", func() {
		var (
			mocks map[string]sqlmock.Sqlmock
			pg    *helpers.PGData
		)

		BeforeEach(func() {
			mocks = make(map[string]sqlmock.Sqlmock)
			db, mock, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks[helpers.DefaultDB] = mock
			db1, mock1, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks["db1"] = mock1
			db2, mock2, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks["db2"] = mock2
			dbsuper, mocksuper, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks["dbsuper"] = mocksuper
			db1super, mock1super, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks["db1super"] = mock1super
			db2super, mock2super, err := sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			mocks["db2super"] = mock2super
			pg = &helpers.PGData{
				Data: helpers.PGCommon{
					DefUser: helpers.User{
						Name:     "defUser",
						Password: "defPassword",
					},
					AdminUser: helpers.User{
						Name:     "superUser",
						Password: "superPassword",
					},
				},
				DBs: []helpers.PGConn{
					helpers.PGConn{
						DB:       db,
						User:     "defUser",
						TargetDB: helpers.DefaultDB,
					},
					helpers.PGConn{
						DB:       db1,
						User:     "defUser",
						TargetDB: "db1",
					},
					helpers.PGConn{
						DB:       db2,
						User:     "defUser",
						TargetDB: "db2",
					},
					helpers.PGConn{
						DB:       dbsuper,
						User:     "superUser",
						TargetDB: helpers.DefaultDB,
					},
					helpers.PGConn{
						DB:       db1super,
						User:     "superUser",
						TargetDB: "db1",
					},
					helpers.PGConn{
						DB:       db2super,
						User:     "superUser",
						TargetDB: "db2",
					},
				},
			}
		})
		AfterEach(func() {
			pg.CloseConnections()
		})
		Context("Run a generic query", func() {
			It("Returns all the lines", func() {
				expected := []string{
					"{\"name\": \"pgadmin1\", \"role\": \"admin\"}",
					"{\"name\": \"pgadmin2\", \"role\": \"admin\"}",
				}
				rows := sqlmock.NewRows(expectedcolumns).
					AddRow(expected[0]).
					AddRow(expected[1])
				query := "SELECT name,role FROM table"
				mocks[helpers.DefaultDB].ExpectQuery(convertQuery(query)).WillReturnRows(rows)
				conn, err := pg.GetDefaultConnection()
				Expect(err).NotTo(HaveOccurred())
				result, err := conn.Run(query)
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).To(Equal(expected))
			})
			It("Properly reports a failure", func() {
				query := "SELECT name,role FROM table"
				mocks[helpers.DefaultDB].ExpectQuery(convertQuery(query)).WillReturnError(genericError)
				conn, err := pg.GetDefaultConnection()
				Expect(err).NotTo(HaveOccurred())
				_, err = conn.Run(query)
				Expect(err).To(MatchError(genericError))
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
		Context("Fail to retrieve env info", func() {
			It("Fails to read postgresql version", func() {
				mockPostgreSQLVersion(helpers.PGVersion{Version: ""}, mocks)
				_, err := pg.GetPostgreSQLVersion()
				Expect(err).To(MatchError(genericError))
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("Fails to read pg_settings", func() {
				mockSettings(nil, mocks)
				_, err := pg.ReadAllSettings()
				Expect(err).To(MatchError(genericError))
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("Fails to list databases", func() {
				mockDatabases(nil, mocks)
				_, err := pg.ListDatabases()
				Expect(err).To(MatchError(genericError))
				if err = mocks["dbsuper"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("Fails to list roles", func() {
				err := mockRoles(nil, mocks)
				Expect(err).NotTo(HaveOccurred())
				_, err = pg.ListRoles()
				Expect(err).To(MatchError(genericError))
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
			It("Fails to convert date to postgres date", func() {
				err := mockDate("xxx", "", mocks)
				Expect(err).NotTo(HaveOccurred())
				_, err = pg.ConvertToPostgresDate("xxx")
				Expect(err).To(MatchError(genericError))
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
		Context("Correctly retrieve env info", func() {
			It("Correctly get postgresql version", func() {
				version := "PostgreSQL 9.4.9"
				mockPostgreSQLVersion(helpers.PGVersion{Version: version}, mocks)
				result, err := pg.GetPostgreSQLVersion()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result.Version).To(Equal(version))
			})
			It("Correctly read pg_settings", func() {
				expected := map[string]string{
					"a1": "a2",
					"b1": "b2",
				}
				mockSettings(expected, mocks)
				result, err := pg.ReadAllSettings()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).NotTo(BeZero())
				Expect(result).To(Equal(expected))
			})
			It("Correctly lists databases without extensions", func() {
				expected := []helpers.PGDatabase{
					helpers.PGDatabase{
						Name:   "db1",
						DBExts: []helpers.PGDatabaseExtensions{},
						Tables: []helpers.PGTable{},
					},
					helpers.PGDatabase{
						Name:   "db2",
						DBExts: []helpers.PGDatabaseExtensions{},
						Tables: []helpers.PGTable{},
					},
				}
				mockDatabases(expected, mocks)
				result, err := pg.ListDatabases()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				if err = mocks["db1"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				if err = mocks["db2"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).To(Equal(expected))
			})
			It("Correctly lists databases with extensions", func() {
				expected := []helpers.PGDatabase{
					helpers.PGDatabase{
						Name: "db1",
						DBExts: []helpers.PGDatabaseExtensions{
							helpers.PGDatabaseExtensions{
								Name: "exta",
							},
							helpers.PGDatabaseExtensions{
								Name: "extb",
							},
						},
						Tables: []helpers.PGTable{},
					},
				}
				mockDatabases(expected, mocks)
				result, err := pg.ListDatabases()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				if err = mocks["db1"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).To(Equal(expected))
			})
			It("Correctly lists databases with tables", func() {
				expected := []helpers.PGDatabase{
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
								SchemaName: "myschema2",
								TableName:  "mytable2",
								TableOwner: "myowner2",
								TableColumns: []helpers.PGTableColumn{
									helpers.PGTableColumn{
										ColumnName: "column3",
										DataType:   "type3",
										Position:   0,
									},
								},
								TableRowsCount: helpers.PGCount{Num: 0},
							},
						},
					},
				}
				mockDatabases(expected, mocks)
				result, err := pg.ListDatabases()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				if err = mocks["db1"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).To(Equal(expected))
			})
			It("Correctly lists roles with properties", func() {
				expected := map[string]helpers.PGRole{
					"role1": helpers.PGRole{
						Name:        "role1",
						Super:       true,
						Inherit:     false,
						CreateRole:  false,
						CreateDb:    true,
						CanLogin:    true,
						Replication: false,
						ConnLimit:   10,
						ValidUntil:  "",
					},
					"role2": helpers.PGRole{
						Name:        "role2",
						Super:       false,
						Inherit:     true,
						CreateRole:  true,
						CreateDb:    false,
						CanLogin:    false,
						Replication: true,
						ConnLimit:   100,
						ValidUntil:  "xxx",
					},
				}
				err := mockRoles(expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				result, err := pg.ListRoles()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).To(Equal(expected))
			})
			It("Correctly retrieves all postgres data", func() {
				expected := helpers.PGOutputData{
					Roles: map[string]helpers.PGRole{
						"pgadmin": helpers.PGRole{
							Name:      "pgadmin",
							CanLogin:  true,
							ConnLimit: 20,
						},
					},
					Databases: []helpers.PGDatabase{
						helpers.PGDatabase{
							Name:   "db1",
							DBExts: []helpers.PGDatabaseExtensions{},
							Tables: []helpers.PGTable{},
						},
					},
					Settings: map[string]string{
						"max_connections": "30",
					},
					Version: helpers.PGVersion{
						Version: "PostgreSQL 9.4.9",
					},
				}
				mockSettings(expected.Settings, mocks)
				mockDatabases(expected.Databases, mocks)
				err := mockRoles(expected.Roles, mocks)
				Expect(err).NotTo(HaveOccurred())
				mockPostgreSQLVersion(expected.Version, mocks)
				result, err := pg.GetData()
				Expect(err).NotTo(HaveOccurred())
				if err = mocks[helpers.DefaultDB].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				if err = mocks["db1"].ExpectationsWereMet(); err != nil {
					Expect(err).NotTo(HaveOccurred())
				}
				Expect(result).NotTo(BeZero())
				Expect(result).To(Equal(expected))
			})
			It("Correctly converts date to postgres date", func() {
				input := "May 5 12:00:00 2017 +1"
				expected := "2017-05-05 11:00:00"
				err := mockDate(input, expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				result, err := pg.ConvertToPostgresDate(input)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expected))
			})
			It("Correctly converts date with quotes to postgres date", func() {
				input := "May 5 12:00:00 2017 +1"
				inputQuotes := fmt.Sprintf("'%s'", input)
				expected := "2017-05-05 11:00:00"
				err := mockDate(input, expected, mocks)
				Expect(err).NotTo(HaveOccurred())
				result, err := pg.ConvertToPostgresDate(inputQuotes)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expected))
			})
		})
	})
	Describe("Load DB", func() {
		var (
			mock     sqlmock.Sqlmock
			pg       *helpers.PGData
			prepared string
		)
		Context("Load DB with a table", func() {

			BeforeEach(func() {
				var db *sql.DB
				var err error
				db, mock, err = sqlmock.New()
				Expect(err).NotTo(HaveOccurred())
				pg = &helpers.PGData{
					Data: helpers.PGCommon{},
					DBs: []helpers.PGConn{
						helpers.PGConn{DB: db, TargetDB: "db1"},
					},
				}
				prepared = `COPY "pgats_table_0" ("column0") FROM STDIN`
				prepared = strings.Replace(prepared, ")", "\\)", -1)
				prepared = strings.Replace(prepared, "(", "\\(", -1)
			})
			AfterEach(func() {
				for _, conn := range pg.DBs {
					conn.DB.Close()
				}
			})
			It("Correctly create the table", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin()
				mock.ExpectPrepare(prepared)
				mock.ExpectExec(prepared).WithArgs("short_string0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).NotTo(HaveOccurred())

				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to create the table", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to begin the connection", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin().WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to prepare the statement", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin()
				mock.ExpectPrepare(prepared).WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to populate row", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin()
				mock.ExpectPrepare(prepared)
				mock.ExpectExec(prepared).WithArgs("short_string0").WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to flush buffered data", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin()
				mock.ExpectPrepare(prepared)
				mock.ExpectExec(prepared).WithArgs("short_string0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("").WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
			It("Fails to commit", func() {
				mock.ExpectExec("CREATE TABLE pgats_table_0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectBegin()
				mock.ExpectPrepare(prepared)
				mock.ExpectExec(prepared).WithArgs("short_string0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(genericError)

				err := pg.CreateAndPopulateTables("db1", helpers.Test1Load)
				Expect(err).To(MatchError(genericError))
				Expect(mock.ExpectationsWereMet()).NotTo(HaveOccurred())
			})
		})
	})
})
