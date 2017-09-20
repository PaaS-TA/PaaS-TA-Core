package helpers

type PropertiesConsul struct {
	Agent *PropertiesConsulAgent
}

type PropertiesConsulAgent struct {
	Services *PropertiesConsulAgentServices `yaml:"services,omitempty"`
	Mode     string                         `yaml:"mode,omitempty"`
}

type PropertiesConsulAgentServices struct {
	Etcd              *PropertiesConsulServicesEtcd `yaml:"etcd,omitempty"`
	Blobstore         interface{}                   `yaml:"blobstore,omitempty"`
	UAA               interface{}                   `yaml:"uaa,omitempty"`
	CloudControllerNG interface{}                   `yaml:"cloud_controller_ng,omitempty"`
	HM9000            interface{}                   `yaml:"hm9000,omitempty"`
	DEA               interface{}                   `yaml:"dea,omitempty"`
	Gorouter          interface{}                   `yaml:"gorouter,omitempty"`
}

type PropertiesConsulServicesEtcd struct {
	Name string `yaml:"name,omitempty"`
}

type PropertiesMetronAgent struct {
	Zone string
}
