package helpers

import "gopkg.in/yaml.v2"

func CreateDiegoTLSMigrationManifest(nonTLSDiegoManifest []byte) ([]byte, error) {
	var manifest Manifest
	err := yaml.Unmarshal(nonTLSDiegoManifest, &manifest)
	if err != nil {
		return []byte{}, err
	}

	for jobName, manifestProperties := range manifest.Properties {
		globalProperties, ok := manifestProperties.(map[interface{}]interface{})
		if !ok {
			continue
		}

		if jobName == "metron_agent" || jobName == "loggregator" {
			globalProperties["etcd"] = etcdConsumerProperties(true)
		}
		manifest.Properties[jobName] = globalProperties
	}

	result, err := yaml.Marshal(manifest)
	if err != nil {
		return []byte{}, err
	}

	return result, nil
}
