package consul

type Config struct {
	DirectorUUID   string
	Name           string
	ConsulSecrets  ConfigSecretsConsul
	DC             string
	Networks       []ConfigNetwork
	TurbulenceHost string
}

type ConfigNetwork struct {
	IPRange string
	Nodes   int
}

type ConfigSecretsConsul struct {
	EncryptKey string
	AgentKey   string
	AgentCert  string
	ServerKey  string
	ServerCert string
	CACert     string
}
