package haproxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"code.cloudfoundry.org/cf-tcp-router/models"
	"code.cloudfoundry.org/cf-tcp-router/utils"

	"code.cloudfoundry.org/cf-tcp-router"
	"code.cloudfoundry.org/lager"
)

type Configurer struct {
	logger             lager.Logger
	baseConfigFilePath string
	configFilePath     string
	configFileLock     *sync.Mutex
	scriptRunner       ScriptRunner
}

func NewHaProxyConfigurer(logger lager.Logger, baseConfigFilePath string, configFilePath string, scriptRunner ScriptRunner) (*Configurer, error) {
	if !utils.FileExists(baseConfigFilePath) {
		return nil, fmt.Errorf("%s: [%s]", cf_tcp_router.ErrRouterConfigFileNotFound, baseConfigFilePath)
	}
	if !utils.FileExists(configFilePath) {
		return nil, fmt.Errorf("%s: [%s]", cf_tcp_router.ErrRouterConfigFileNotFound, configFilePath)
	}
	return &Configurer{
		logger:             logger,
		baseConfigFilePath: baseConfigFilePath,
		configFilePath:     configFilePath,
		configFileLock:     new(sync.Mutex),
		scriptRunner:       scriptRunner,
	}, nil
}

func (h *Configurer) Configure(routingTable models.RoutingTable) error {
	h.configFileLock.Lock()
	defer h.configFileLock.Unlock()

	err := h.createConfigBackup()
	if err != nil {
		return err
	}

	cfgContent, err := ioutil.ReadFile(h.baseConfigFilePath)
	if err != nil {
		h.logger.Error("failed-reading-base-config-file", err, lager.Data{"base-config-file": h.baseConfigFilePath})
		return err
	}
	var buff bytes.Buffer
	_, err = buff.Write(cfgContent)
	if err != nil {
		h.logger.Error("failed-copying-config-file", err, lager.Data{"config-file": h.configFilePath})
		return err
	}

	for key, entry := range routingTable.Entries {
		cfgContent, err = h.getListenConfiguration(key, entry)
		if err != nil {
			continue
		}
		_, err = buff.Write(cfgContent)
		if err != nil {
			h.logger.Error("failed-writing-to-buffer", err)
			return err
		}
	}

	h.logger.Info("writing-config", lager.Data{"num-bytes": buff.Len()})
	err = h.writeToConfig(buff.Bytes())
	if err != nil {
		return err
	}

	if h.scriptRunner != nil {
		h.logger.Info("running-script")
		err = h.scriptRunner.Run()
		if err != nil {
			h.logger.Error("failed-to-run-script", err)
			return err
		}
	}
	return nil
}

func (h *Configurer) getListenConfiguration(key models.RoutingKey, entry models.RoutingTableEntry) ([]byte, error) {
	var buff bytes.Buffer
	_, err := buff.WriteString("\n")
	if err != nil {
		h.logger.Error("failed-writing-to-buffer", err)
		return nil, err
	}

	var listenCfgStr string
	listenCfgStr, err = RoutingTableEntryToHaProxyConfig(key, entry)
	if err != nil {
		h.logger.Error("failed-marshaling-routing-table-entry", err)
		return nil, err
	}

	_, err = buff.WriteString(listenCfgStr)
	if err != nil {
		h.logger.Error("failed-writing-to-buffer", err)
		return nil, err
	}
	return buff.Bytes(), nil
}

func (h *Configurer) createConfigBackup() error {
	h.logger.Debug("reading-config-file", lager.Data{"config-file": h.configFilePath})
	cfgContent, err := ioutil.ReadFile(h.configFilePath)
	if err != nil {
		h.logger.Error("failed-reading-base-config-file", err, lager.Data{"config-file": h.configFilePath})
		return err
	}
	backupConfigFileName := fmt.Sprintf("%s.bak", h.configFilePath)
	err = utils.WriteToFile(cfgContent, backupConfigFileName)
	if err != nil {
		h.logger.Error("failed-to-backup-config", err, lager.Data{"config-file": h.configFilePath})
		return err
	}
	return nil
}

func (h *Configurer) writeToConfig(cfgContent []byte) error {
	tmpConfigFileName := fmt.Sprintf("%s.tmp", h.configFilePath)
	err := utils.WriteToFile(cfgContent, tmpConfigFileName)
	if err != nil {
		h.logger.Error("failed-to-write-temp-config", err, lager.Data{"temp-config-file": tmpConfigFileName})
		return err
	}

	err = os.Rename(tmpConfigFileName, h.configFilePath)
	if err != nil {
		h.logger.Error(
			"failed-renaming-temp-config-file",
			err,
			lager.Data{"config-file": h.configFilePath, "temp-config-file": tmpConfigFileName})
		return err
	}
	return nil
}
