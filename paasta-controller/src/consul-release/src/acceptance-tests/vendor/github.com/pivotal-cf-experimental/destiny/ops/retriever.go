package ops

import (
	"errors"

	yaml "gopkg.in/yaml.v2"
)

type InstanceGroup struct {
	Name      string
	Instances int
	Lifecycle string
}

func ManifestName(manifest string) (string, error) {
	var manifestStruct struct {
		Name string
	}

	err := yaml.Unmarshal([]byte(manifest), &manifestStruct)
	if err != nil {
		return "", err
	}

	if manifestStruct.Name == "" {
		return "", errors.New("could not find name in manifest")
	}

	return manifestStruct.Name, nil
}

func InstanceGroups(manifest string) ([]InstanceGroup, error) {
	var manifestStruct struct {
		InstanceGroups []InstanceGroup `yaml:"instance_groups"`
	}

	err := yaml.Unmarshal([]byte(manifest), &manifestStruct)
	if err != nil {
		return []InstanceGroup{}, err
	}

	return manifestStruct.InstanceGroups, nil
}
