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
	MigratedFrom       []InstanceGroupMigratedFrom `yaml:"migrated_from,omitempty"`
	Properties         *JobProperties              `yaml:"properties,omitempty"`
}

type InstanceGroupMigratedFrom struct {
	Name string `yaml:"name"`
	AZ   string `yaml:"az"`
}

type InstanceGroupNetwork JobNetwork

type InstanceGroupJob struct {
	Name    string `yaml:"name"`
	Release string `yaml:"release"`
}
