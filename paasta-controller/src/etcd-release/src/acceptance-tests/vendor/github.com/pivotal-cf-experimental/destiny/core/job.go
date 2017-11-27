package core

type Job struct {
	Instances      int            `yaml:"instances"`
	Lifecycle      string         `yaml:"lifecycle,omitempty"`
	Name           string         `yaml:"name"`
	Networks       []JobNetwork   `yaml:"networks"`
	ResourcePool   string         `yaml:"resource_pool"`
	Templates      []JobTemplate  `yaml:"templates"`
	PersistentDisk int            `yaml:"persistent_disk,omitempty"`
	Properties     *JobProperties `yaml:"properties,omitempty"`
	Update         *JobUpdate     `yaml:"update,omitempty"`
}

type JobUpdate struct {
	MaxInFlight int `yaml:"max_in_flight"`
}

type JobNetwork struct {
	Name      string   `yaml:"name"`
	StaticIPs []string `yaml:"static_ips"`
}

type JobTemplate struct {
	Name     string      `yaml:"name"`
	Release  string      `yaml:"release"`
	Consumes JobConsumes `yaml:"consumes,omitempty"`
}

type JobProperties struct {
	Consul           *JobPropertiesConsul           `yaml:"consul,omitempty"`
	Etcd             *JobPropertiesEtcd             `yaml:"etcd,omitempty"`
	EtcdTestConsumer *JobPropertiesEtcdTestConsumer `yaml:"etcd_testconsumer,omitempty"`
}

type JobPropertiesConsul struct {
	Agent       JobPropertiesConsulAgent `yaml:"agent"`
	CACert      string                   `yaml:"ca_cert,omitempty"`
	ServerCert  string                   `yaml:"server_cert,omitempty"`
	ServerKey   string                   `yaml:"server_key,omitempty"`
	AgentCert   string                   `yaml:"agent_cert,omitempty"`
	AgentKey    string                   `yaml:"agent_key,omitempty"`
	EncryptKeys []string                 `yaml:"encrypt_keys,omitempty"`
}

type JobPropertiesEtcd struct {
	Machines                        []string                   `yaml:"machines"`
	PeerRequireSSL                  bool                       `yaml:"peer_require_ssl"`
	RequireSSL                      bool                       `yaml:"require_ssl"`
	HeartbeatIntervalInMilliseconds int                        `yaml:"heartbeat_interval_in_milliseconds"`
	Cluster                         []JobPropertiesEtcdCluster `yaml:"cluster,omitempty"`
	AdvertiseURLsDNSSuffix          string                     `yaml:"advertise_urls_dns_suffix,omitempty"`
	CACert                          string                     `yaml:"ca_cert,omitempty"`
	ClientCert                      string                     `yaml:"client_cert,omitempty"`
	ClientKey                       string                     `yaml:"client_key,omitempty"`
	PeerCACert                      string                     `yaml:"peer_ca_cert,omitempty"`
	PeerCert                        string                     `yaml:"peer_cert,omitempty"`
	PeerKey                         string                     `yaml:"peer_key,omitempty"`
	ServerCert                      string                     `yaml:"server_cert,omitempty"`
	ServerKey                       string                     `yaml:"server_key,omitempty"`
}

type JobPropertiesEtcdTestConsumer struct {
	Etcd JobPropertiesEtcdTestConsumerEtcd `yaml:"etcd"`
}

type JobPropertiesEtcdTestConsumerEtcd struct {
	Machines   []string `yaml:"machines"`
	RequireSSL bool     `yaml:"require_ssl"`
	CACert     string   `yaml:"ca_cert"`
	ClientCert string   `yaml:"client_cert"`
	ClientKey  string   `yaml:"client_key"`
}

type JobPropertiesEtcdCluster struct {
	Name      string `yaml:"name"`
	Instances int    `yaml:"instances"`
}

type JobPropertiesConsulAgent struct {
	Domain     string                           `yaml:"domain,omitempty"`
	Datacenter string                           `yaml:"datacenter,omitempty"`
	Servers    JobPropertiesConsulAgentServers  `yaml:"servers,omitempty"`
	Mode       string                           `yaml:"mode,omitempty"`
	LogLevel   string                           `yaml:"log_level,omitempty"`
	Services   JobPropertiesConsulAgentServices `yaml:"services,omitempty"`
}

type JobPropertiesConsulAgentServers struct {
	Lan []string `yaml:"lan"`
}

type JobPropertiesConsulAgentServices map[string]JobPropertiesConsulAgentService

type JobPropertiesConsulAgentService struct {
	Name  string                                `yaml:"name,omitempty"`
	Check *JobPropertiesConsulAgentServiceCheck `yaml:"check,omitempty"`
	Tags  []string                              `yaml:"tags,omitempty"`
}

type JobPropertiesConsulAgentServiceCheck struct {
	Name     string `yaml:"name"`
	Script   string `yaml:"script,omitempty"`
	Interval string `yaml:"interval,omitempty"`
}

func (j Job) HasTemplate(name, release string) bool {
	for _, template := range j.Templates {
		if template.Name == name && template.Release == release {
			return true
		}
	}
	return false
}
