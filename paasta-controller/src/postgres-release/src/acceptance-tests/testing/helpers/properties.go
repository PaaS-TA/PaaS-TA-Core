package helpers

import yaml "gopkg.in/yaml.v2"

type Properties struct {
	Databases PgProperties `yaml:"databases"`
}
type PgProperties struct {
	Port                  int                   `yaml:"port"`
	Databases             []PgDBProperties      `yaml:"databases,omitempty"`
	Roles                 []PgRoleProperties    `yaml:"roles,omitempty"`
	MaxConnections        int                   `yaml:"max_connections"`
	LogLinePrefix         string                `yaml:"log_line_prefix"`
	CollectStatementStats bool                  `yaml:"collect_statement_statistics"`
	MonitTimeout          int                   `yaml:"monit_timeout,omitempty"`
	AdditionalConfig      PgAdditionalConfigMap `yaml:"additional_config,omitempty"`
	TLS                   PgTLS                 `yaml:"tls,omitempty"`
}

type PgDBProperties struct {
	CITExt bool   `yaml:"citext"`
	Name   string `yaml:"name"`
}

type PgRoleProperties struct {
	Name        string   `yaml:"name"`
	Password    string   `yaml:"password"`
	Permissions []string `yaml:"permissions,omitempty"`
}

type PgTLS struct {
	PrivateKey  string `yaml:"private_key"`
	Certificate string `yaml:"certificate"`
	CA          string `yaml:"ca"`
}

type PgAdditionalConfig interface{}
type PgAdditionalConfigMap map[string]PgAdditionalConfig

var defaultPgProperties = PgProperties{
	LogLinePrefix:         "%m: ",
	CollectStatementStats: false,
	MaxConnections:        500,
}

type ManifestProperties struct {
	ByJob map[string][]Properties
}

func decodeProperties(yamlData []byte) (Properties, error) {
	var props Properties
	var err error

	props = Properties{Databases: defaultPgProperties}
	err = yaml.Unmarshal(yamlData, &props)
	if err != nil {
		return Properties{}, err
	}
	return props, nil
}

func (mp *ManifestProperties) LoadJobProperties(jobName string, yamlData []byte) error {
	props, err := decodeProperties(yamlData)
	if err != nil {
		return err
	}
	if mp.ByJob == nil {
		mp.ByJob = make(map[string][]Properties)
	}
	mp.ByJob[jobName] = append(mp.ByJob[jobName], props)
	return nil
}

func (mp ManifestProperties) GetJobProperties(jobName string) []Properties {
	result := mp.ByJob[jobName]
	return result
}
