package internal

import (
	"archive/tar"
	"errors"
	"fmt"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager"
)

const MAX_RESULT_SIZE = 1024 * 10

var ErrResultFileTooLarge = errors.New(
	fmt.Sprintf("result file is too large (over %d bytes)", MAX_RESULT_SIZE),
)

//go:generate counterfeiter -o fake_internal/fake_container_delegate.go container_delegate.go ContainerDelegate

type ContainerDelegate interface {
	GetContainer(logger lager.Logger, guid string) (executor.Container, bool)
	RunContainer(logger lager.Logger, req *executor.RunRequest) bool
	StopContainer(logger lager.Logger, guid string) bool
	DeleteContainer(logger lager.Logger, guid string) bool
	FetchContainerResultFile(logger lager.Logger, guid string, filename string) (string, error)
}

type containerDelegate struct {
	client executor.Client
}

func NewContainerDelegate(client executor.Client) ContainerDelegate {
	return &containerDelegate{
		client: client,
	}
}

func (d *containerDelegate) GetContainer(logger lager.Logger, guid string) (executor.Container, bool) {
	logger.Debug("fetch-container")
	container, err := d.client.GetContainer(logger, guid)
	if err != nil {
		logInfoOrError(logger, "failed-fetch-container", err)
		return container, false
	}
	logger.Debug("succeeded-fetch-container")
	return container, true
}

func (d *containerDelegate) RunContainer(logger lager.Logger, req *executor.RunRequest) bool {
	logger.Info("running-container")
	err := d.client.RunContainer(logger, req)
	if err != nil {
		logInfoOrError(logger, "failed-running-container", err)
		d.DeleteContainer(logger, req.Guid)
		return false
	}
	logger.Info("succeeded-running-container")
	return true
}

func (d *containerDelegate) StopContainer(logger lager.Logger, guid string) bool {
	logger.Info("stopping-container")
	err := d.client.StopContainer(logger, guid)
	if err != nil {
		logInfoOrError(logger, "failed-stopping-container", err)
		return false
	}
	logger.Info("succeeded-stopping-container")
	return true
}

func (d *containerDelegate) DeleteContainer(logger lager.Logger, guid string) bool {
	logger.Info("deleting-container")
	err := d.client.DeleteContainer(logger, guid)
	if err != nil {
		logInfoOrError(logger, "failed-deleting-container", err)
		return false
	}
	logger.Info("succeeded-deleting-container")
	return true
}

func (d *containerDelegate) FetchContainerResultFile(logger lager.Logger, guid string, filename string) (string, error) {
	logger.Info("fetching-container-result")
	stream, err := d.client.GetFiles(logger, guid, filename)
	if err != nil {
		logInfoOrError(logger, "failed-fetching-container-result-stream-from-executor", err)
		return "", err
	}

	defer stream.Close()

	tarReader := tar.NewReader(stream)

	_, err = tarReader.Next()
	if err != nil {
		return "", err
	}

	buf := make([]byte, MAX_RESULT_SIZE+1)
	n, err := tarReader.Read(buf)
	if n > MAX_RESULT_SIZE {
		logInfoOrError(logger, "failed-fetching-container-result-too-large", err)
		return "", ErrResultFileTooLarge
	}

	logger.Info("succeeded-fetching-container-result")
	return string(buf[:n]), nil
}

func logInfoOrError(logger lager.Logger, msg string, err error) {
	if err == executor.ErrContainerNotFound {
		logger.Info(msg, lager.Data{"error": err.Error()})
	} else {
		logger.Error(msg, err)
	}
}
