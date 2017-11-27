package helpers

import (
	"io/ioutil"
	"sort"

	yaml "gopkg.in/yaml.v2"
)

var defaultVersionsFile string = "../versions.yml"

type PostgresReleaseVersions struct {
	sortedKeys []int
	Versions   map[int]string `yaml:"versions"`
	Old        int            `yaml:"old"`
	Older      int            `yaml:"older"`
}

func NewPostgresReleaseVersions(versionFile string) (PostgresReleaseVersions, error) {
	var versions PostgresReleaseVersions

	if versionFile == "" {
		versionFile = defaultVersionsFile
	}

	data, err := ioutil.ReadFile(versionFile)
	if err != nil {
		return PostgresReleaseVersions{}, err
	}
	if err := yaml.Unmarshal(data, &versions); err != nil {
		return PostgresReleaseVersions{}, err
	}
	for key := range versions.Versions {
		versions.sortedKeys = append(versions.sortedKeys, key)
	}
	sort.Ints(versions.sortedKeys)

	return versions, nil
}

func (v PostgresReleaseVersions) GetOldVersion() int {
	return v.Old
}

func (v PostgresReleaseVersions) GetOlderVersion() int {
	return v.Older
}
func (v PostgresReleaseVersions) GetLatestVersion() int {
	return v.sortedKeys[len(v.sortedKeys)-1]
}
func (v PostgresReleaseVersions) GetPostgreSQLVersion(key int) string {
	return v.Versions[key]
}
