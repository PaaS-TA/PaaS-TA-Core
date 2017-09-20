package chaperon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/consul-release/src/confab/config"
)

type ConfigWriter struct {
	dir    string
	logger logger
}

func NewConfigWriter(dir string, logger logger) ConfigWriter {
	return ConfigWriter{
		dir:    dir,
		logger: logger,
	}
}

func (w ConfigWriter) Write(cfg config.Config) error {
	var err error

	nodeName := cfg.Consul.Agent.NodeName

	if nodeName == "" {
		nodeName, err = nodeNameFor(cfg.Path.DataDir, cfg.Node)
		if err != nil {
			w.logger.Error("config-writer.write.determine-node-name.failed", err)
			return err
		}
	}

	w.logger.Info("config-writer.write.determine-node-name", lager.Data{
		"node-name": nodeName,
	})

	w.logger.Info("config-writer.write.generate-configuration")
	consulConfig := config.GenerateConfiguration(cfg, w.dir, nodeName)

	data, err := json.Marshal(&consulConfig)
	if err != nil {
		return err
	}

	w.logger.Info("config-writer.write.write-file", lager.Data{
		"config": consulConfig,
	})
	err = ioutil.WriteFile(filepath.Join(w.dir, "config.json"), data, os.ModePerm)
	if err != nil {
		w.logger.Error("config-writer.write.write-file.failed", errors.New(err.Error()))
		return err
	}

	w.logger.Info("config-writer.write.success")
	return nil
}

type nodeName struct {
	NodeName string `json:"node_name"`
}

func nodeNameFor(dataDir string, node config.ConfigNode) (string, error) {
	var oldNodeName nodeName
	var name string

	_, err := os.Stat(dataDir)
	if err != nil {
		return "", err
	}

	buf, err := ioutil.ReadFile(filepath.Join(dataDir, "node-name.json"))
	switch {
	case err == nil:
		err = json.Unmarshal(buf, &oldNodeName)
		if err != nil {
			return "", err
		}
		name = oldNodeName.NodeName
	case os.IsNotExist(err):
		name = strings.Replace(node.Name, "_", "-", -1)
		name = fmt.Sprintf("%s-%d", name, node.Index)

		nodeNameJSON, err := json.Marshal(nodeName{NodeName: name})
		if err != nil {
			return "", err
		}

		err = ioutil.WriteFile(filepath.Join(dataDir, "node-name.json"), []byte(nodeNameJSON), os.ModePerm)
		if err != nil {
			return "", err
		}
	default:
		return "", err
	}

	return name, nil
}
