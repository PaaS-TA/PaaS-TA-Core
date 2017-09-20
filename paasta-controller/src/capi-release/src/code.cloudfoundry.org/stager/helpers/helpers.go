package helpers

import (
	"encoding/json"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

func BuildDockerStagingData(dockerImage string) (*json.RawMessage, error) {
	rawJsonBytes, err := json.Marshal(cc_messages.DockerStagingData{
		DockerImageUrl: dockerImage,
	})
	if err != nil {
		return nil, err
	}
	jsonRawMessage := json.RawMessage(rawJsonBytes)
	return &jsonRawMessage, nil
}
