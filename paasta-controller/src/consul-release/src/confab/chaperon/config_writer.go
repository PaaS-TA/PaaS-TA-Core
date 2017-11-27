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
		nodeName, err = getNodeName(cfg.Path.DataDir, cfg.Node.Name, cfg.Node.Index)
		if err != nil {
			w.logger.Error("config-writer.write.determine-node-name.failed", err)
			return err
		}
	} else {
		nodeName, err = getNodeName(cfg.Path.DataDir, nodeName, cfg.Node.Index)
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

type node struct {
	NodeName string `json:"node_name"`
}

func getNodeName(dataDir string, nodeName string, nodeIndex int) (string, error) {
	var oldNode node
	var name string

	_, err := os.Stat(dataDir)
	if err != nil {
		return "", err
	}

	buf, err := ioutil.ReadFile(filepath.Join(dataDir, "node-name.json"))
	switch {
	case err == nil:
		err = json.Unmarshal(buf, &oldNode)
		if err != nil {
			return "", err
		}
		name = oldNode.NodeName
	case os.IsNotExist(err):
		name = strings.Replace(nodeName, "_", "-", -1)
		name = fmt.Sprintf("%s-%d", name, nodeIndex)

		nodeNameJSON, err := json.Marshal(node{NodeName: name})
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
