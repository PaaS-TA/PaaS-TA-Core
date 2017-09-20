package core

type InstanceGroup struct {
	Instances          int                         `yaml:"instances"`
	Name               string                      `yaml:"name"`
	AZs                []string                    `yaml:"azs"`
	Networks           []InstanceGroupNetwork      `yaml:"networks"`
	VMType             string                      `yaml:"vm_type"`
	Stemcell           string                      `yaml:"stemcell"`
	PersistentDiskType string                      `yaml:"persistent_disk_type,omitempty"`
	Update             Update                      `yaml:"update,omitempty"`
	Jobs               []InstanceGroupJob          `yaml:"jobs"`
	MigratedFrom       []InstanceGroupMigratedFrom `yaml:"migrated_from"`
	Properties         InstanceGroupProperties     `yaml:"properties,omitempty"`
}

type InstanceGroupMigratedFrom struct {
	Name string `yaml:"name"`
	AZ   string `yaml:"az"`
}

type InstanceGroupProperties struct {
	Consul InstanceGroupPropertiesConsul `yaml:"consul"`
}

type InstanceGroupPropertiesConsul struct {
	Agent InstanceGroupPropertiesConsulAgent `yaml:"agent"`
}

type InstanceGroupPropertiesConsulAgent struct {
	Mode     string                                               `yaml:"mode"`
	LogLevel string                                               `yaml:"log_level"`
	Services map[string]InstanceGroupPropertiesConsulAgentService `yaml:"services"`
}

type InstanceGroupPropertiesConsulAgentService struct {
	Name  string                                         `yaml:"name,omitempty"`
	Check InstanceGroupPropertiesConsulAgentServiceCheck `yaml:"check,omitempty"`
	Tags  []string                                       `yaml:"tags,omitempty"`
}

type InstanceGroupPropertiesConsulAgentServiceCheck struct {
	Name     string `yaml:"name,omitempty"`
	Script   string `yaml:"script,omitempty"`
	Interval string `yaml:"interval,omitempty"`
}

type InstanceGroupNetwork struct {
	Name      string   `yaml:"name"`
	StaticIPs []string `yaml:"static_ips"`
}

type InstanceGroupJob struct {
	Name    string `yaml:"name"`
	Release string `yaml:"release"`
}
