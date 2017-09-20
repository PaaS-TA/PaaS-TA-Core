package turbulence

type Config struct {
	DirectorUUID string
	Name         string
	IPRange      string
	BOSH         ConfigBOSH
}

type ConfigBOSH struct {
	Target             string
	Username           string
	Password           string
	DirectorCACert     string
	PersistentDiskType string
	VMType             string
}
