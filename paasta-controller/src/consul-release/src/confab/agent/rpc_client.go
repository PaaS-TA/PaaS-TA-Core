package agent

import (
	"errors"

	"github.com/hashicorp/consul/command/agent"
)

const keyringToken = ""

type RPCClient struct {
	agent.RPCClient
}

func HandleRPCErrors(info []agent.KeyringInfo) error {
	for _, msg := range info {
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
	}
	return nil
}

func (c RPCClient) ListKeys() ([]string, error) {
	response, err := c.RPCClient.ListKeys("")
	if err != nil {
		return nil, err
	}

	err = HandleRPCErrors(response.Info)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, keyEntry := range response.Keys {
		if keyEntry.Pool == "LAN" {
			keys = append(keys, keyEntry.Key)
		}
	}

	return keys, nil
}

func (c RPCClient) InstallKey(key string) error {
	response, err := c.RPCClient.InstallKey(key, keyringToken)
	if err != nil {
		return err
	}

	err = HandleRPCErrors(response.Info)
	if err != nil {
		return err
	}

	return nil
}

func (c RPCClient) UseKey(key string) error {
	response, err := c.RPCClient.UseKey(key, keyringToken)
	if err != nil {
		return err
	}

	err = HandleRPCErrors(response.Info)
	if err != nil {
		return err
	}

	return nil
}

func (c RPCClient) RemoveKey(key string) error {
	response, err := c.RPCClient.RemoveKey(key, keyringToken)
	if err != nil {
		return err
	}

	err = HandleRPCErrors(response.Info)
	if err != nil {
		return err
	}

	return nil
}
