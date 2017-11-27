package helpers

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	cfgtypes "github.com/cloudfoundry/config-server/types"
)

const DefaultDB = "postgres"

type PGData struct {
	Data PGCommon
	DBs  []PGConn
}

type User struct {
	Name        string
	Password    string
	Certificate string
	Key         string
}

type PGCommon struct {
	Address     string
	Port        int
	SSLMode     string
	SSLRootCert string
	DefUser     User
	AdminUser   User
	CertUser    User
	UseCert     bool
}

type PGConn struct {
	TargetDB string
	User     string
	password string
	DB       *sql.DB
}

type PGSetting struct {
	Name    string `json:"name"`
	Setting string `json:"setting"`
	VarType string `json:"vartype"`
}
type PGDatabase struct {
	Name   string `json:"datname"`
	DBExts []PGDatabaseExtensions
	Tables []PGTable
}
type PGDatabaseExtensions struct {
	Name string `json:"extname"`
}
type PGTable struct {
	SchemaName     string `json:"schemaname"`
	TableName      string `json:"tablename"`
	TableOwner     string `json:"tableowner"`
	TableColumns   []PGTableColumn
	TableRowsCount PGCount
}
type PGTableColumn struct {
	ColumnName string `json:"column_name"`
	DataType   string `json:"data_type"`
	Position   int    `json:"ordinal_position"`
}
type PGCount struct {
	Num int `json:"count"`
}
type PGVersion struct {
	Version string `json:"version"`
}
type PGRole struct {
	Name        string `json:"rolname"`
	Super       bool   `json:"rolsuper"`
	Inherit     bool   `json:"rolinherit"`
	CreateRole  bool   `json:"rolcreaterole"`
	CreateDb    bool   `json:"rolcreatedb"`
	CanLogin    bool   `json:"rolcanlogin"`
	Replication bool   `json:"rolreplication"`
	ConnLimit   int    `json:"rolconnlimit"`
	ValidUntil  string `json:"rolvaliduntil"`
}

type PGOutputData struct {
	Roles     map[string]PGRole
	Databases []PGDatabase
	Settings  map[string]string
	Version   PGVersion
}

const GetSettingsQuery = "SELECT * FROM pg_settings"
const ListRolesQuery = "SELECT * from pg_roles"
const ListDatabasesQuery = "SELECT datname from pg_database where datistemplate=false"
const ListDBExtensionsQuery = "SELECT extname from pg_extension"
const ConvertToDateCommand = "SELECT '%s'::timestamptz"
const ListTablesQuery = "SELECT * from pg_catalog.pg_tables where schemaname not like 'pg_%' and schemaname != 'information_schema'"
const ListTableColumnsQuery = "SELECT column_name, data_type, ordinal_position FROM information_schema.columns WHERE table_schema = '%s' AND table_name = '%s' order by ordinal_position asc"
const CountTableRowsQuery = "SELECT COUNT(*) FROM %s"
const GetPostgreSQLVersionQuery = "SELECT version()"
const QueryResultAsJson = "SELECT row_to_json(t) from (%s) as t;"

const NoConnectionAvailableErr = "No connections available"
const MissingDBAddressErr = "Database address not specified"
const MissingDBPortErr = "Database port not specified"
const MissingDefaultUserErr = "Default user not specified"
const MissingDefaultPasswordErr = "Default password not specified"
const NoSuperUserProvidedErr = "No super user provided"
const IncorrectSSLModeErr = "Incorrect SSL mode specified"
const MissingSSLRootCertErr = "SSL Root Certificate missing"
const MissingCertUserErr = "No user specified to authenticate with certificates"
const MissingCertCertErr = "No certificate specified for cert user"
const MissingCertKeyErr = "No private key specified for cert user's certificate"

func GetFormattedQuery(query string) string {
	return fmt.Sprintf(QueryResultAsJson, query)
}

func NewPostgres(props PGCommon) (PGData, error) {
	var pg PGData
	if props.SSLMode == "" {
		props.SSLMode = "disable"
	}
	if err := checkSSLMode(props.SSLMode, props.SSLRootCert); err != nil {
		return PGData{}, err
	}
	if props.Address == "" {
		return PGData{}, errors.New(MissingDBAddressErr)
	}
	if props.Port == 0 {
		return PGData{}, errors.New(MissingDBPortErr)
	}
	if props.DefUser == (User{}) {
		return PGData{}, errors.New(MissingDefaultUserErr)
	}
	if props.DefUser.Name == "" {
		return PGData{}, errors.New(MissingDefaultUserErr)
	}
	if props.DefUser.Password == "" {
		return PGData{}, errors.New(MissingDefaultPasswordErr)
	}
	pg.Data = props
	return pg, nil
}

func checkSSLMode(sslmode string, sslrootcert string) error {
	var strong_sslmodes = [...]string{"verify-ca", "verify-full"}
	var valid_sslmodes = [...]string{"disable", "require", "verify-ca", "verify-full"}
	for _, valid_mode := range valid_sslmodes {
		if valid_mode == sslmode {
			for _, strong_mode := range strong_sslmodes {
				if strong_mode == sslmode && sslrootcert == "" {
					return errors.New(MissingSSLRootCertErr)
				}
			}
			return nil
		}
	}
	return errors.New(IncorrectSSLModeErr)
}

func (pg PGData) getDefaultUser() User {
	if pg.Data.UseCert {
		return pg.Data.CertUser
	} else {
		return pg.Data.DefUser
	}
}

func (pg PGData) checkCertUser() error {
	if pg.Data.CertUser == (User{}) {
		return errors.New(MissingCertUserErr)
	}
	if pg.Data.CertUser.Certificate == "" {
		return errors.New(MissingCertCertErr)
	}
	if pg.Data.CertUser.Key == "" {
		return errors.New(MissingCertKeyErr)
	}
	return nil
}
func (pg *PGData) SetCertUserCertificates(user string, certs interface{}) error {
	if user == "" {
		if pg.Data.UseCert {
			return errors.New(MissingCertUserErr)
		}
		pg.Data.CertUser = User{}
	} else {
		clientCertPath, err := WriteFile(certs.(cfgtypes.CertResponse).Certificate)
		if err != nil {
			return err
		}
		clientKeyPath, err := WriteFile(certs.(cfgtypes.CertResponse).PrivateKey)
		if err != nil {
			return err
		}
		if pg.Data.CertUser.Certificate != "" {
			os.Remove(pg.Data.CertUser.Certificate)
		}
		if pg.Data.CertUser.Key != "" {
			os.Remove(pg.Data.CertUser.Key)
		}
		pg.Data.CertUser.Name = user
		pg.Data.CertUser.Certificate = clientCertPath
		pg.Data.CertUser.Key = clientKeyPath
	}
	return nil
}

func (pg *PGData) UseCertAuthentication(useCert bool) error {
	if err := pg.checkCertUser(); err != nil {
		return err
	}
	pg.Data.UseCert = useCert
	pg.CloseConnections()
	return nil
}
func (pg *PGData) ChangeSSLMode(sslmode string, sslrootcert string) error {
	if err := checkSSLMode(sslmode, sslrootcert); err != nil {
		return err
	}
	pg.Data.SSLMode = sslmode
	pg.Data.SSLRootCert = sslrootcert
	pg.CloseConnections()
	return nil
}

func (pg PGData) buildConnectionData(dbname string, user User) string {
	result := fmt.Sprintf("dbname=%s user=%s host=%s port=%d sslmode=%s", dbname, user.Name, pg.Data.Address, pg.Data.Port, pg.Data.SSLMode)
	if pg.Data.SSLRootCert != "" {
		result = fmt.Sprintf("%s sslrootcert=%s", result, pg.Data.SSLRootCert)
	}
	if user.Password != "" {
		result = fmt.Sprintf("%s password=%s", result, user.Password)
	} else {
		result = fmt.Sprintf("%s sslcert=%s sslkey=%s", result, user.Certificate, user.Key)
	}
	return result
}

func (pg *PGData) OpenConnection(dbname string, user User) (PGConn, error) {
	var newConn PGConn
	var err error

	connectionData := pg.buildConnectionData(dbname, user)
	newConn.DB, err = sql.Open("postgres", connectionData)
	if err != nil {
		return PGConn{}, err
	}
	err = newConn.DB.Ping()
	if err != nil {
		return PGConn{}, err
	}
	newConn.User = user.Name
	newConn.password = user.Password
	newConn.TargetDB = dbname
	newConn.DB.SetMaxIdleConns(10)
	pg.DBs = append(pg.DBs, newConn)
	return newConn, nil
}
func (pg *PGData) CloseConnections() {
	for _, conn := range pg.DBs {
		conn.DB.Close()
	}
	pg.DBs = nil
}
func (pg *PGData) GetDefaultConnection() (PGConn, error) {
	return pg.GetDBConnection(DefaultDB)
}

func (pg *PGData) GetDBSuperUserConnection(dbname string) (PGConn, error) {
	if pg.Data.AdminUser == (User{}) ||
		pg.Data.AdminUser.Name == "" ||
		pg.Data.AdminUser.Password == "" {
		return PGConn{}, errors.New(NoSuperUserProvidedErr)
	}
	conn, err := pg.GetDBConnectionForUser(dbname, pg.Data.AdminUser)
	if err != nil {
		conn, err = pg.OpenConnection(dbname, pg.Data.AdminUser)
		if err != nil {
			return PGConn{}, err
		}
	}
	return conn, nil
}
func (pg *PGData) GetSuperUserConnection() (PGConn, error) {
	return pg.GetDBSuperUserConnection(DefaultDB)
}

func (pg *PGData) GetDBConnection(dbname string) (PGConn, error) {
	result, err := pg.GetDBConnectionForUser(dbname, pg.getDefaultUser())
	if (PGConn{}) == result {
		result, err = pg.OpenConnection(dbname, pg.getDefaultUser())
		if err != nil {
			return PGConn{}, err
		}
	}
	return result, nil
}
func (pg PGData) GetDBConnectionForUser(dbname string, user User) (PGConn, error) {
	if len(pg.DBs) == 0 {
		return PGConn{}, errors.New(NoConnectionAvailableErr)
	}
	var result PGConn
	for _, conn := range pg.DBs {
		if conn.TargetDB == dbname {
			if user.Name == "" || conn.User == user.Name {
				result = conn
				break
			}
		}
	}
	if (PGConn{}) == result {
		return PGConn{}, errors.New(NoConnectionAvailableErr)
	}
	return result, nil
}

func (pg PGConn) Run(query string) ([]string, error) {
	var result []string
	if rows, err := pg.DB.Query(GetFormattedQuery(query)); err != nil {
		return nil, err
	} else {
		defer rows.Close()
		for rows.Next() {
			var jsonRow string
			if err := rows.Scan(&jsonRow); err != nil {
				break
			}
			result = append(result, jsonRow)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (pg PGConn) Exec(query string) error {
	if _, err := pg.DB.Exec(query); err != nil {
		return err
	}
	return nil
}

func (pg PGData) CreateAndPopulateTables(dbName string, loadType LoadType) error {

	conn, err := pg.GetDBConnection(dbName)
	if err != nil {
		return err
	}
	tables := GetSampleLoad(loadType)

	for _, table := range tables {
		err = conn.Exec(table.PrepareCreate())
		if err != nil {
			return err
		}
		txn, err := conn.DB.Begin()
		if err != nil {
			return err
		}

		stmt, err := txn.Prepare(table.PrepareStatement())
		if err != nil {
			return err
		}

		for i := 0; i < table.NumRows; i++ {
			_, err = stmt.Exec(table.PrepareRow(i)...)
			if err != nil {
				return err
			}
		}

		_, err = stmt.Exec()
		if err != nil {
			return err
		}

		err = stmt.Close()
		if err != nil {
			return err
		}

		err = txn.Commit()
		if err != nil {
			return err
		}
	}

	return err
}

func (pg PGData) ReadAllSettings() (map[string]string, error) {
	result := make(map[string]string)
	conn, err := pg.GetDefaultConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Run(GetSettingsQuery)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out := PGSetting{}
		err = json.Unmarshal([]byte(row), &out)
		if err != nil {
			return nil, err
		}
		result[out.Name] = out.Setting
	}
	return result, nil
}
func (pg PGData) GetPostgreSQLVersion() (PGVersion, error) {
	var result PGVersion

	conn, err := pg.GetDefaultConnection()
	if err != nil {
		return PGVersion{}, err
	}
	rows, err := conn.Run(GetPostgreSQLVersionQuery)
	if err != nil {
		return PGVersion{}, err
	}
	err = json.Unmarshal([]byte(rows[0]), &result)
	if err != nil {
		return PGVersion{}, err
	}
	return result, nil
}
func (pg PGData) ListDatabases() ([]PGDatabase, error) {
	var result []PGDatabase
	conn, err := pg.GetSuperUserConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Run(ListDatabasesQuery)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out := PGDatabase{}
		err = json.Unmarshal([]byte(row), &out)
		if err != nil {
			return nil, err
		}
		result = append(result, out)
	}
	for idx, database := range result {
		result[idx].DBExts, err = pg.ListDatabaseExtensions(database.Name)
		if err != nil {
			return nil, err
		}
		result[idx].Tables, err = pg.ListDatabaseTables(database.Name)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (pg PGData) ListDatabaseExtensions(dbName string) ([]PGDatabaseExtensions, error) {
	conn, err := pg.GetDBSuperUserConnection(dbName)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Run(ListDBExtensionsQuery)
	if err != nil {
		return nil, err
	}
	extensionsList := []PGDatabaseExtensions{}
	for _, row := range rows {
		out := PGDatabaseExtensions{}
		err = json.Unmarshal([]byte(row), &out)
		if err != nil {
			return nil, err
		}
		extensionsList = append(extensionsList, out)
	}
	return extensionsList, nil
}
func (pg PGData) ListDatabaseTables(dbName string) ([]PGTable, error) {
	conn, err := pg.GetDBSuperUserConnection(dbName)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Run(ListTablesQuery)
	if err != nil {
		return nil, err
	}
	tableList := []PGTable{}
	for _, row := range rows {
		tableData := PGTable{}
		err = json.Unmarshal([]byte(row), &tableData)
		if err != nil {
			return nil, err
		}
		tableData.TableColumns = []PGTableColumn{}
		colRows, err := conn.Run(fmt.Sprintf(ListTableColumnsQuery, tableData.SchemaName, tableData.TableName))
		if err != nil {
			return nil, err
		}
		for _, colRow := range colRows {
			colData := PGTableColumn{}
			err = json.Unmarshal([]byte(colRow), &colData)
			if err != nil {
				return nil, err
			}
			tableData.TableColumns = append(tableData.TableColumns, colData)
		}
		countRows, err := conn.Run(fmt.Sprintf(CountTableRowsQuery, tableData.TableName))
		if err != nil {
			return nil, err
		}
		count := PGCount{}
		err = json.Unmarshal([]byte(countRows[0]), &count)
		if err != nil {
			return nil, err
		}
		tableData.TableRowsCount = count

		tableList = append(tableList, tableData)
	}
	return tableList, nil
}
func (pg PGData) ListRoles() (map[string]PGRole, error) {
	result := make(map[string]PGRole)
	conn, err := pg.GetDefaultConnection()
	if err != nil {
		return nil, err
	}
	rows, err := conn.Run(ListRolesQuery)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out := PGRole{}
		err = json.Unmarshal([]byte(row), &out)
		if err != nil {
			return nil, err
		}
		result[out.Name] = out
	}
	return result, nil
}

func (pg PGData) ConvertToPostgresDate(inputDate string) (string, error) {
	type ConvertedDate struct {
		Date string `json:"timestamptz"`
	}
	result := ConvertedDate{}
	inputDate = strings.TrimLeft(inputDate, "'\"")
	inputDate = strings.TrimRight(inputDate, "'\"")
	conn, err := pg.GetDefaultConnection()
	if err != nil {
		return "", err
	}
	rows, err := conn.Run(fmt.Sprintf(ConvertToDateCommand, inputDate))
	if err != nil {
		return "", err
	}
	err = json.Unmarshal([]byte(rows[0]), &result)
	if err != nil {
		return "", err
	}
	return result.Date, nil
}

func (pg PGData) GetData() (PGOutputData, error) {
	var result PGOutputData
	var err error
	result.Settings, err = pg.ReadAllSettings()
	if err != nil {
		return PGOutputData{}, err
	}
	result.Databases, err = pg.ListDatabases()
	if err != nil {
		return PGOutputData{}, err
	}
	result.Roles, err = pg.ListRoles()
	if err != nil {
		return PGOutputData{}, err
	}
	result.Version, err = pg.GetPostgreSQLVersion()
	if err != nil {
		return PGOutputData{}, err
	}
	return result, nil
}

func (o PGOutputData) CopyData() (PGOutputData, error) {
	var to PGOutputData
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	dec := gob.NewDecoder(&buffer)
	err := enc.Encode(o)
	if err != nil {
		return PGOutputData{}, err
	}
	err = dec.Decode(&to)
	if err != nil {
		return PGOutputData{}, err
	}
	return to, nil
}
