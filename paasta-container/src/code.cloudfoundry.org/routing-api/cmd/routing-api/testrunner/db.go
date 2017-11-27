package testrunner

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var etcdVersion = "etcdserver\":\"2.1.1"

type DbAllocator interface {
	Create() (string, error)
	Reset() error
	Delete() error
	ConnectionString() string
}

type mysqlAllocator struct {
	sqlDB      *sql.DB
	schemaName string
}

type postgresAllocator struct {
	sqlDB      *sql.DB
	schemaName string
}

func randSchemaName() string {
	return fmt.Sprintf("test%d", rand.Int())
}

func NewPostgresAllocator() DbAllocator {
	return &postgresAllocator{schemaName: randSchemaName()}
}
func NewMySQLAllocator() DbAllocator {
	return &mysqlAllocator{schemaName: randSchemaName()}
}

type etcdAllocator struct {
	port        int
	etcdAdapter storeadapter.StoreAdapter
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
}

func NewEtcdAllocator(port int) DbAllocator {
	return &etcdAllocator{port: port}
}

func (a *postgresAllocator) ConnectionString() string {
	return "postgres://postgres:@localhost/?sslmode=disable"
}

func (a *postgresAllocator) Create() (string, error) {
	var err error
	a.sqlDB, err = sql.Open("postgres", a.ConnectionString())
	if err != nil {
		return "", err
	}
	err = a.sqlDB.Ping()
	if err != nil {
		return "", err
	}

	for i := 0; i < 5; i++ {
		dbExists, err := a.sqlDB.Exec(fmt.Sprintf("SELECT * FROM pg_database WHERE datname='%s'", a.schemaName))
		rowsAffected, err := dbExists.RowsAffected()
		if err != nil {
			return "", err
		}
		if rowsAffected == 0 {
			_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
			if err != nil {
				return "", err
			}
			return a.schemaName, nil
		} else {
			a.schemaName = randSchemaName()
		}
	}
	return "", errors.New("Failed to create unique database ")
}

func (a *postgresAllocator) Reset() error {
	_, err := a.sqlDB.Exec(fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
	WHERE datname = '%s'`, a.schemaName))
	_, err = a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	if err != nil {
		return err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	return err
}

func (a *postgresAllocator) Delete() error {
	defer func() {
		_ = a.sqlDB.Close()
	}()
	_, err := a.sqlDB.Exec(fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity
	WHERE datname = '%s'`, a.schemaName))
	if err != nil {
		return err
	}
	_, err = a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	return err
}

func (a *mysqlAllocator) ConnectionString() string {
	return "root:password@/"
}

func (a *mysqlAllocator) Create() (string, error) {
	var err error
	a.sqlDB, err = sql.Open("mysql", a.ConnectionString())
	if err != nil {
		return "", err
	}
	err = a.sqlDB.Ping()
	if err != nil {
		return "", err
	}

	for i := 0; i < 5; i++ {
		dbExists, err := a.sqlDB.Exec(fmt.Sprintf("SHOW DATABASES LIKE '%s'", a.schemaName))
		rowsAffected, err := dbExists.RowsAffected()
		if err != nil {
			return "", err
		}
		if rowsAffected == 0 {
			_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
			if err != nil {
				return "", err
			}
			return a.schemaName, nil
		} else {
			a.schemaName = randSchemaName()
		}
	}
	return "", errors.New("Failed to create unique database ")
}

func (a *mysqlAllocator) Reset() error {
	_, err := a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	if err != nil {
		return err
	}

	_, err = a.sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", a.schemaName))
	return err
}

func (a *mysqlAllocator) Delete() error {
	defer func() {
		_ = a.sqlDB.Close()
	}()
	_, err := a.sqlDB.Exec(fmt.Sprintf("DROP DATABASE %s", a.schemaName))
	return err
}

func (e *etcdAllocator) Create() (string, error) {
	e.etcdRunner = etcdstorerunner.NewETCDClusterRunner(e.port, 1, nil)
	e.etcdRunner.Start()

	etcdVersionUrl := e.etcdRunner.NodeURLS()[0] + "/version"
	resp, err := http.Get(etcdVersionUrl)
	defer func() {
		_ = resp.Body.Close()
	}()
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// response body: {"etcdserver":"2.1.1","etcdcluster":"2.1.0"}
	if !strings.Contains(string(body), etcdVersion) {
		return "", errors.New("Incorrect etcd version")
	}

	e.etcdAdapter = e.etcdRunner.Adapter(nil)

	return e.ConnectionString(), nil
}

func (e *etcdAllocator) ConnectionString() string {
	return fmt.Sprintf("http://127.0.0.1:%d", e.port)
}

func (e *etcdAllocator) Reset() error {
	e.etcdRunner.Reset()
	return nil
}

func (e *etcdAllocator) Delete() error {
	err := e.etcdAdapter.Disconnect()
	if err != nil {
		return err
	}
	e.etcdRunner.Reset()
	e.etcdRunner.KillWithFire()
	return nil
}
