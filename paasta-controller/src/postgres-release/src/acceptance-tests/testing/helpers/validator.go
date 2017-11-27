package helpers

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Validator struct {
	ManifestProps     Properties
	PostgresData      PGOutputData
	PG                PGData
	PostgreSQLVersion string
}

const WrongPostreSQLVersionError = "Actual PostgreSQL version %s should be %s"
const MissingDatabaseValidationError = "Database %s has not been created"
const ExtraDatabaseValidationError = "Extra database %s has been created"
const MissingExtensionValidationError = "Extension %s for database %s has not been created"
const ExtraExtensionValidationError = "Extra extension %s for database %s has been created"
const MissingRoleValidationError = "Role %s has not been created"
const ExtraRoleValidationError = "Extra role %s has been created"
const IncorrectRolePrmissionValidationError = "Incorrect permissions for role %s"
const IncorrectSettingValidationError = "Incorrect value %v instead of %v for setting %s"
const MissingSettingValidationError = "Missing setting %s"

type PGDBSorter []PGDatabase

func (a PGDBSorter) Len() int      { return len(a) }
func (a PGDBSorter) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PGDBSorter) Less(i, j int) bool {
	return a[j].Name == DefaultDB || (a[i].Name != DefaultDB && a[i].Name < a[j].Name)
}

type PgDBPropsSorter []PgDBProperties

func (a PgDBPropsSorter) Len() int           { return len(a) }
func (a PgDBPropsSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PgDBPropsSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

type PGTableSorter []PGTable

func (a PGTableSorter) Len() int      { return len(a) }
func (a PGTableSorter) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PGTableSorter) Less(i, j int) bool {
	return a[i].SchemaName < a[j].SchemaName || (a[i].SchemaName == a[j].SchemaName && a[i].TableName < a[j].TableName)
}

type PGColumnSorter []PGTableColumn

func (a PGColumnSorter) Len() int           { return len(a) }
func (a PGColumnSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a PGColumnSorter) Less(i, j int) bool { return a[i].Position < a[j].Position }

func NewValidator(props Properties, pgData PGOutputData, pg PGData, postgresqlVersion string) Validator {
	return Validator{
		PostgreSQLVersion: postgresqlVersion,
		ManifestProps:     props,
		PostgresData:      pgData,
		PG:                pg,
	}
}

func (v Validator) ValidatePostgreSQLVersion() error {
	if !strings.HasPrefix(v.PostgresData.Version.Version, v.PostgreSQLVersion) {
		return errors.New(fmt.Sprintf(WrongPostreSQLVersionError, v.PostgresData.Version.Version, v.PostgreSQLVersion))
	}
	return nil
}

func (v Validator) ValidateDatabases() error {
	actual := v.PostgresData.Databases
	expected := v.ManifestProps.Databases.Databases
	sort.Sort(PGDBSorter(actual))
	sort.Sort(PgDBPropsSorter(expected))
	for idx, actualDB := range actual {
		if actualDB.Name == DefaultDB && idx > len(expected)-1 {
			break
		}
		if idx > len(expected)-1 || actualDB.Name < expected[idx].Name {
			if actualDB.Name != DefaultDB {
				return errors.New(fmt.Sprintf(ExtraDatabaseValidationError, actualDB.Name))
			}
		}
		if actualDB.Name == DefaultDB || actualDB.Name > expected[idx].Name {
			return errors.New(fmt.Sprintf(MissingDatabaseValidationError, expected[idx].Name))
		}
		extMap := map[string]bool{
			"plpgsql":            false,
			"pgcrypto":           false,
			"citext":             false,
			"pg_stat_statements": false,
		}
		for _, dbExt := range actualDB.DBExts {
			if _, ok := extMap[dbExt.Name]; !ok {
				return errors.New(fmt.Sprintf(ExtraExtensionValidationError, dbExt.Name, expected[idx].Name))
			}
			extMap[dbExt.Name] = true
		}
		if !extMap["pgcrypto"] {
			return errors.New(fmt.Sprintf(MissingExtensionValidationError, "pgcrypto", expected[idx].Name))
		}
		if expected[idx].CITExt && !extMap["citext"] {
			return errors.New(fmt.Sprintf(MissingExtensionValidationError, "citext", expected[idx].Name))
		} else if !expected[idx].CITExt && extMap["citext"] {
			return errors.New(fmt.Sprintf(ExtraExtensionValidationError, "citext", expected[idx].Name))
		}
		if v.ManifestProps.Databases.CollectStatementStats && !extMap["pg_stat_statements"] {
			return errors.New(fmt.Sprintf(MissingExtensionValidationError, "pg_stat_statements", expected[idx].Name))
		} else if !v.ManifestProps.Databases.CollectStatementStats && extMap["pg_stat_statements"] {
			return errors.New(fmt.Sprintf(ExtraExtensionValidationError, "pg_stat_statements", expected[idx].Name))
		}
	}
	return nil
}
func (v Validator) ValidateRoles() error {
	var err error
	actual := v.PostgresData.Roles
	expected := v.ManifestProps.Databases.Roles

	for _, expectedRole := range expected {
		actualRole, ok := actual[expectedRole.Name]
		if !ok {
			return errors.New(fmt.Sprintf(MissingRoleValidationError, expectedRole.Name))
		}

		defaultRole := PGRole{
			Name:        actualRole.Name,
			Super:       false,
			Inherit:     true,
			CreateRole:  false,
			CreateDb:    false,
			CanLogin:    true,
			Replication: false,
			ConnLimit:   -1,
			ValidUntil:  "",
		}
		for _, elem := range expectedRole.Permissions {
			switch {
			case elem == "SUPERUSER":
				defaultRole.Super = true
			case elem == "CREATEDB":
				defaultRole.CreateDb = true
			case elem == "CREATEROLE":
				defaultRole.CreateRole = true
			case elem == "NOINHERIT":
				defaultRole.Inherit = false
			case elem == "NOLOGIN":
				defaultRole.CanLogin = false
			case elem == "REPLICATION":
				defaultRole.Replication = true
			case strings.Contains(elem, "CONNECTION LIMIT"):
				value, err := strconv.Atoi(strings.SplitAfter(elem, "CONNECTION LIMIT ")[1])
				if err != nil {
					return err
				}
				defaultRole.ConnLimit = value
			case strings.Contains(elem, "VALID UNTIL"):
				defaultRole.ValidUntil, err = v.PG.ConvertToPostgresDate(strings.SplitAfter(elem, "VALID UNTIL ")[1])
				if err != nil {
					return err
				}
			default:
			}
		}
		if defaultRole != actualRole {
			return errors.New(fmt.Sprintf(IncorrectRolePrmissionValidationError, actualRole.Name))
		}
	}
	return nil
}

// TODO cover all setting types
// PostgreSQL stores setting as formatted strings
// the value in the postgresql.conf may not match the value from pg_settings view
// e.g. the shared_buffer property is stored as an int but can be specified as 128MB
func (v Validator) MatchSetting(key string, value interface{}) error {
	settings := v.PostgresData.Settings
	stringValue := fmt.Sprintf("%v", value)
	expected, ok := settings[key]
	if !ok {
		return errors.New(fmt.Sprintf(MissingSettingValidationError, key))
	} else if expected != stringValue {
		return errors.New(fmt.Sprintf(IncorrectSettingValidationError, stringValue, expected, key))
	}
	return nil
}
func (v Validator) ValidateSettings() error {
	var err error
	props := v.ManifestProps.Databases
	for key, value := range props.AdditionalConfig {
		err = v.MatchSetting(key, value)
		if err != nil {
			return err
		}
	}
	err = v.MatchSetting("port", props.Port)
	if err != nil {
		return err
	}
	err = v.MatchSetting("max_connections", props.MaxConnections)
	if err != nil {
		return err
	}
	err = v.MatchSetting("log_line_prefix", props.LogLinePrefix)
	if err != nil {
		return err
	}
	return nil
}
func (v Validator) ValidateAll() error {
	var err error
	err = v.ValidateDatabases()
	if err != nil {
		return err
	}
	err = v.ValidateRoles()
	if err != nil {
		return err
	}
	err = v.ValidateSettings()
	if err != nil {
		return err
	}
	err = v.ValidatePostgreSQLVersion()
	if err != nil {
		return err
	}
	return err
}
func (v Validator) CompareTablesTo(data PGOutputData) bool {

	aDBs := v.PostgresData.Databases
	eDBs := data.Databases

	result := (len(aDBs) == len(eDBs))
	if result {
		sort.Sort(PGDBSorter(aDBs))
		sort.Sort(PGDBSorter(eDBs))
		for i, db := range aDBs {
			if db.Name != eDBs[i].Name ||
				(len(db.Tables) != len(eDBs[i].Tables)) {
				result = false
				break
			} else if len(db.Tables) > 0 {
				sort.Sort(PGTableSorter(db.Tables))
				sort.Sort(PGTableSorter(eDBs[i].Tables))
				for j, table := range db.Tables {
					vs := eDBs[i].Tables[j]
					if vs.SchemaName != table.SchemaName ||
						vs.TableName != table.TableName ||
						vs.TableOwner != table.TableOwner ||
						vs.TableRowsCount.Num != table.TableRowsCount.Num ||
						len(vs.TableColumns) != len(table.TableColumns) {
						result = false
						break
					}
					sort.Sort(PGColumnSorter(vs.TableColumns))
					sort.Sort(PGColumnSorter(table.TableColumns))
					for k, col := range table.TableColumns {
						vsc := eDBs[i].Tables[j].TableColumns[k]
						if vsc.ColumnName != col.ColumnName ||
							vsc.DataType != col.DataType ||
							vsc.Position != col.Position {
							result = false
							break
						}
					}
				}
			}
		}
	}
	return result
}
