package consul

type ConfigV2 struct {
	DirectorUUID       string
	Name               string
	AZs                []ConfigAZ
	PersistentDiskType string
	VMType             string
}

type ConfigAZ struct {
	Name    string
	IPRange string
	Nodes   int
}
