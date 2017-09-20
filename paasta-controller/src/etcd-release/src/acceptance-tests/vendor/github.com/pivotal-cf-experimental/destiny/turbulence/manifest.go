package turbulence

import (
	"github.com/pivotal-cf-experimental/destiny/core"
	"github.com/pivotal-cf-experimental/destiny/iaas"
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	DirectorUUID  string              `yaml:"director_uuid"`
	Name          string              `yaml:"name"`
	Jobs          []core.Job          `yaml:"jobs"`
	Properties    Properties          `yaml:"properties"`
	Update        core.Update         `yaml:"update"`
	Compilation   core.Compilation    `yaml:"compilation"`
	Networks      []core.Network      `yaml:"networks"`
	Releases      []core.Release      `yaml:"releases"`
	ResourcePools []core.ResourcePool `yaml:"resource_pools"`
}

func NewManifest(config Config, iaasConfig iaas.Config) (Manifest, error) {
	turbulenceRelease := core.Release{
		Name:    "turbulence",
		Version: "latest",
	}

	cidrBlock, err := core.ParseCIDRBlock(config.IPRange)
	if err != nil {
		return Manifest{}, err
	}

	cloudProperties := iaasConfig.NetworkSubnet(config.IPRange)
	cpi := iaasConfig.CPI()

	cpiRelease := core.Release{
		Name:    cpi.ReleaseName,
		Version: "latest",
	}

	turbulenceNetwork := core.Network{
		Name: "turbulence",
		Subnets: []core.NetworkSubnet{{
			CloudProperties: cloudProperties,
			Gateway:         cidrBlock.GetFirstIP().Add(1).String(),
			Range:           cidrBlock.String(),
			Reserved:        []string{cidrBlock.Range(2, 3), cidrBlock.GetLastIP().String()},
			Static:          []string{cidrBlock.Range(4, cidrBlock.CIDRSize-5)},
		}},
		Type: "manual",
	}

	compilation := core.Compilation{
		Network:             turbulenceNetwork.Name,
		ReuseCompilationVMs: true,
		Workers:             3,
		CloudProperties:     iaasConfig.Compilation(),
	}

	turbulenceResourcePool := core.ResourcePool{
		Name:    "turbulence",
		Network: turbulenceNetwork.Name,
		Stemcell: core.ResourcePoolStemcell{
			Name:    iaasConfig.Stemcell(),
			Version: "latest",
		},
		CloudProperties: iaasConfig.ResourcePool(config.IPRange),
	}

	update := core.Update{
		Canaries:        1,
		CanaryWatchTime: "1000-180000",
		MaxInFlight:     1,
		Serial:          true,
		UpdateWatchTime: "1000-180000",
	}

	staticIps, err := turbulenceNetwork.StaticIPsFromRange(17)
	if err != nil {
		return Manifest{}, err
	}

	apiJob := core.Job{
		Instances: 1,
		Name:      "api",
		Networks: []core.JobNetwork{{
			Name:      turbulenceNetwork.Name,
			StaticIPs: []string{staticIps[16]},
		}},
		PersistentDisk: 1024,
		ResourcePool:   turbulenceResourcePool.Name,
		Templates: []core.JobTemplate{
			{
				Name:    "turbulence_api",
				Release: turbulenceRelease.Name,
			},
			{
				Name:    cpi.JobName,
				Release: cpiRelease.Name,
			},
		},
	}

	directorCACert := BOSHDirectorCACert
	if config.BOSH.DirectorCACert != "" {
		directorCACert = config.BOSH.DirectorCACert
	}

	iaasProperties := iaasConfig.Properties(staticIps[16])
	turbulenceProperties := Properties{
		WardenCPI: iaasProperties.WardenCPI,
		AWS:       iaasProperties.AWS,
		Registry:  iaasProperties.Registry,
		Blobstore: iaasProperties.Blobstore,
		Agent:     iaasProperties.Agent,
		TurbulenceAPI: &PropertiesTurbulenceAPI{
			Certificate: APICertificate,
			CPIJobName:  cpi.JobName,
			Director: PropertiesTurbulenceAPIDirector{
				CACert:   directorCACert,
				Host:     config.BOSH.Target,
				Password: config.BOSH.Password,
				Username: config.BOSH.Username,
			},
			Password:   DefaultPassword,
			PrivateKey: APIPrivateKey,
		},
	}

	return Manifest{
		DirectorUUID:  config.DirectorUUID,
		Name:          config.Name,
		Releases:      []core.Release{turbulenceRelease, cpiRelease},
		ResourcePools: []core.ResourcePool{turbulenceResourcePool},
		Compilation:   compilation,
		Update:        update,
		Jobs:          []core.Job{apiJob},
		Networks:      []core.Network{turbulenceNetwork},
		Properties:    turbulenceProperties,
	}, nil
}

func (m Manifest) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

func FromYAML(manifestYAML []byte) (Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(manifestYAML, &m); err != nil {
		return m, err
	}
	return m, nil
}
