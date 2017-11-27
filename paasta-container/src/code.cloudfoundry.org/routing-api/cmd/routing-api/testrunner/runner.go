package testrunner

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/cf-tcp-router/utils"

	"github.com/tedsuo/ifrit/ginkgomon"
)

var dbEnv = os.Getenv("DB")

type Args struct {
	Port       uint16
	ConfigPath string
	DevMode    bool
	IP         string
}

func (args Args) ArgSlice() []string {
	return []string{
		"-port", strconv.Itoa(int(args.Port)),
		"-ip", args.IP,
		"-config", args.ConfigPath,
		"-logLevel=debug",
		"-devMode=" + strconv.FormatBool(args.DevMode),
	}
}

func NewDbAllocator(etcdPort int) DbAllocator {
	var dbAllocator DbAllocator
	switch dbEnv {
	case "etcd":
		dbAllocator = NewEtcdAllocator(etcdPort)
	case "postgres":
		dbAllocator = NewPostgresAllocator()
	default:
		dbAllocator = NewMySQLAllocator()
	}
	return dbAllocator
}

func NewRoutingAPIArgs(ip string, port uint16, dbId, consulUrl string) (Args, error) {
	configPath, err := createConfig(dbId, consulUrl)
	if err != nil {
		return Args{}, err
	}
	return Args{
		Port:       port,
		IP:         ip,
		ConfigPath: configPath,
		DevMode:    true,
	}, nil
}

func New(binPath string, args Args) *ginkgomon.Runner {
	return ginkgomon.New(ginkgomon.Config{
		Name:              "routing-api",
		Command:           exec.Command(binPath, args.ArgSlice()...),
		StartCheck:        "routing-api.started",
		StartCheckTimeout: 30 * time.Second,
	})
}

func createConfig(dbId, consulUrl string) (string, error) {
	var configBytes []byte
	configFile, err := ioutil.TempFile("", "routing-api-config")
	if err != nil {
		return "", err
	}
	configFilePath := configFile.Name()

	switch dbEnv {
	case "etcd":
		etcdConfigStr := `log_guid: "my_logs"
uaa_verification_key: "-----BEGIN PUBLIC KEY-----

      MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d

      KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX

      qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug

      spULZVNRxq7veq/fzwIDAQAB

      -----END PUBLIC KEY-----"

uuid: "routing-api-uuid"
debug_address: "1.2.3.4:1234"
metron_config:
  address: "1.2.3.4"
  port: "4567"
metrics_reporting_interval: "500ms"
statsd_endpoint: "localhost:8125"
statsd_client_flush_interval: "10ms"
system_domain: "example.com"
router_groups:
- name: "default-tcp"
  type: "tcp"
  reservable_ports: "1024-65535"
etcd:
  node_urls: ["%s"]
consul_cluster:
  servers: "%s"
  retry_interval: 50ms`
		configBytes = []byte(fmt.Sprintf(etcdConfigStr, dbId, consulUrl))
	case "postgres":
		postgresConfigStr := `log_guid: "my_logs"
uaa_verification_key: "-----BEGIN PUBLIC KEY-----

      MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d

      KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX

      qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug

      spULZVNRxq7veq/fzwIDAQAB

      -----END PUBLIC KEY-----"

uuid: "routing-api-uuid"
debug_address: "1.2.3.4:1234"
metron_config:
  address: "1.2.3.4"
  port: "4567"
metrics_reporting_interval: "500ms"
statsd_endpoint: "localhost:8125"
statsd_client_flush_interval: "10ms"
system_domain: "example.com"
router_groups:
- name: "default-tcp"
  type: "tcp"
  reservable_ports: "1024-65535"
sqldb:
  username: "postgres"
  password: ""
  schema: "%s"
  port: 5432
  host: "localhost"
  type: "postgres"
consul_cluster:
  servers: "%s"
  retry_interval: 50ms`
		configBytes = []byte(fmt.Sprintf(postgresConfigStr, dbId, consulUrl))
	default:
		mysqlConfigStr := `log_guid: "my_logs"
uaa_verification_key: "-----BEGIN PUBLIC KEY-----

      MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d

      KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX

      qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug

      spULZVNRxq7veq/fzwIDAQAB

      -----END PUBLIC KEY-----"

uuid: "routing-api-uuid"
debug_address: "1.2.3.4:1234"
metron_config:
  address: "1.2.3.4"
  port: "4567"
metrics_reporting_interval: "500ms"
statsd_endpoint: "localhost:8125"
statsd_client_flush_interval: "10ms"
system_domain: "example.com"
router_groups:
- name: "default-tcp"
  type: "tcp"
  reservable_ports: "1024-65535"
sqldb:
  username: "root"
  password: "password"
  schema: "%s"
  port: 3306
  host: "localhost"
  type: "mysql"
consul_cluster:
  servers: "%s"
  retry_interval: 50ms`
		configBytes = []byte(fmt.Sprintf(mysqlConfigStr, dbId, consulUrl))
	}

	err = utils.WriteToFile(configBytes, configFilePath)
	return configFilePath, err
}
