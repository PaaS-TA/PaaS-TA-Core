package core

type Compilation struct {
	CloudProperties     CompilationCloudProperties `yaml:"cloud_properties"`
	Network             string                     `yaml:"network"`
	ReuseCompilationVMs bool                       `yaml:"reuse_compilation_vms"`
	Workers             int                        `yaml:"workers"`
}

type CompilationCloudProperties struct {
	InstanceType     string                                   `yaml:"instance_type,omitempty"`
	AvailabilityZone string                                   `yaml:"availability_zone,omitempty"`
	EphemeralDisk    *CompilationCloudPropertiesEphemeralDisk `yaml:"ephemeral_disk,omitempty"`
}

type CompilationCloudPropertiesEphemeralDisk struct {
	Size int    `yaml:"size"`
	Type string `yaml:"type"`
}

type Release struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Stemcell struct {
	Alias   string
	OS      string `yaml:"os,omitempty"`
	Version string
	Name    string
}

type ResourcePool struct {
	CloudProperties ResourcePoolCloudProperties `yaml:"cloud_properties"`
	Name            string                      `yaml:"name"`
	Network         string                      `yaml:"network"`
	Stemcell        ResourcePoolStemcell        `yaml:"stemcell"`
}

type ResourcePoolCloudProperties struct {
	InstanceType     string                                    `yaml:"instance_type,omitempty"`
	AvailabilityZone string                                    `yaml:"availability_zone,omitempty"`
	EphemeralDisk    *ResourcePoolCloudPropertiesEphemeralDisk `yaml:"ephemeral_disk,omitempty"`
}

type ResourcePoolCloudPropertiesEphemeralDisk struct {
	Size int    `yaml:"size"`
	Type string `yaml:"type"`
}

type ResourcePoolStemcell struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Update struct {
	Canaries        int    `yaml:"canaries,omitempty"`
	CanaryWatchTime string `yaml:"canary_watch_time,omitempty"`
	MaxInFlight     int    `yaml:"max_in_flight"`
	Serial          bool   `yaml:"serial,omitempty"`
	UpdateWatchTime string `yaml:"update_watch_time,omitempty"`
}

type PropertiesBlobstore struct {
	Address string                   `yaml:"address"`
	Port    int                      `yaml:"port"`
	Agent   PropertiesBlobstoreAgent `yaml:"agent"`
}

type PropertiesBlobstoreAgent struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type PropertiesAgent struct {
	Mbus string `yaml:"mbus"`
}

type PropertiesRegistry struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Port     int    `yaml:"port"`
}

type PropertiesTurbulenceAgent struct {
	API PropertiesTurbulenceAgentAPI
}

type ConsulTestConsumer struct {
	NameServer string `yaml:"nameserver"`
}

type PropertiesTurbulenceAgentAPI struct {
	Host     string
	Password string
	CACert   string `yaml:"ca_cert"`
}
