package cloudconfig

import (
	"fmt"

	"github.com/pivotal-cf-experimental/destiny/core"

	"gopkg.in/yaml.v2"
)

type Config struct {
	AZs []ConfigAZ
}

type ConfigAZ struct {
	IPRange   string
	StaticIPs int
}

type CloudConfig struct {
	AZs         []AZ        `yaml:"azs"`
	VMTypes     []VMType    `yaml:"vm_types"`
	DiskTypes   []DiskType  `yaml:"disk_types"`
	Compilation Compilation `yaml:"compilation"`
	Networks    []Network   `yaml:"networks"`
}

type AZ struct {
	Name string `yaml:"name"`
}

type VMType struct {
	Name string `yaml:"name"`
}

type DiskType struct {
	Name     string `yaml:"name"`
	DiskSize int    `yaml:"disk_size"`
}

type Compilation struct {
	Workers             int    `yaml:"workers"`
	ReuseCompilationVMs bool   `yaml:"reuse_compilation_vms"`
	AZ                  string `yaml:"az"`
	VMType              string `yaml:"vm_type"`
	Network             string `yaml:"network"`
}

type Network struct {
	Name    string   `yaml:"name"`
	Subnets []Subnet `yaml:"subnets"`
	Type    string   `yaml:"type"`
}

type Subnet struct {
	CloudProperties SubnetCloudProperties `yaml:"cloud_properties"`
	Range           string                `yaml:"range"`
	Gateway         string                `yaml:"gateway"`
	AZ              string                `yaml:"az"`
	Reserved        []string              `yaml:"reserved"`
	Static          []string              `yaml:"static"`
}

type SubnetCloudProperties struct {
	Name string
}

const (
	gatewayIPRangeIndex = 1
)

func NewWardenCloudConfig(config Config) (CloudConfig, error) {
	vmTypes := []VMType{
		{
			Name: "default",
		},
	}

	diskTypes := []DiskType{
		{
			Name:     "default",
			DiskSize: 1024,
		},
	}

	compilation := Compilation{
		Workers:             3,
		ReuseCompilationVMs: true,
		AZ:                  "z1",
		VMType:              "default",
		Network:             "private",
	}

	azs := []AZ{}
	subnets := []Subnet{}

	for i, cfgAZ := range config.AZs {
		azName := fmt.Sprintf("z%d", i+1)
		azs = append(azs, AZ{
			Name: azName,
		})

		cidrBlock, err := core.ParseCIDRBlock(cfgAZ.IPRange)
		if err != nil {
			return CloudConfig{}, err
		}

		subnets = append(subnets, Subnet{
			CloudProperties: SubnetCloudProperties{
				Name: "random",
			},
			Range:    cidrBlock.String(),
			Gateway:  cidrBlock.GetFirstIP().Add(gatewayIPRangeIndex).String(),
			AZ:       azName,
			Reserved: []string{cidrBlock.Range(2, 3), cidrBlock.GetLastIP().String()},
			Static:   []string{cidrBlock.Range(4, cidrBlock.CIDRSize-5)},
		})
	}

	networks := []Network{
		{
			Name:    "private",
			Subnets: subnets,
			Type:    "manual",
		},
	}

	return CloudConfig{
		AZs:         azs,
		VMTypes:     vmTypes,
		DiskTypes:   diskTypes,
		Compilation: compilation,
		Networks:    networks,
	}, nil
}

func (c CloudConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}
