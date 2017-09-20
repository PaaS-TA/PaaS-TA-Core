package turbulence

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
)

type Properties struct {
	WardenCPI     *iaas.PropertiesWardenCPI `yaml:"warden_cpi,omitempty"`
	AWS           *iaas.PropertiesAWS       `yaml:"aws,omitempty"`
	Registry      *core.PropertiesRegistry  `yaml:"registry,omitempty"`
	Blobstore     *core.PropertiesBlobstore `yaml:"blobstore,omitempty"`
	Agent         *core.PropertiesAgent     `yaml:"agent,omitempty"`
	TurbulenceAPI *PropertiesTurbulenceAPI  `yaml:"turbulence_api,omitempty"`
}

type PropertiesTurbulenceAPI struct {
	Certificate string                          `yaml:"certificate"`
	CPIJobName  string                          `yaml:"cpi_job_name"`
	Director    PropertiesTurbulenceAPIDirector `yaml:"director"`
	Password    string                          `yaml:"password"`
	PrivateKey  string                          `yaml:"private_key"`
}

type PropertiesTurbulenceAPIDirector struct {
	CACert   string `yaml:"ca_cert"`
	Host     string `yaml:"host"`
	Password string `yaml:"password"`
	Username string `yaml:"username"`
}
