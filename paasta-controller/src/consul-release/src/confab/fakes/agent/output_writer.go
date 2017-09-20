package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
)

type OutputWriter struct {
	filepath      string
	data          OutputData
	callCountChan chan string
}

type ConsulConfig struct {
	Server    bool `json:"server"`
	Bootstrap bool `json:"bootstrap"`
}

type OutputData struct {
	Args                []string
	ConsulConfig        ConsulConfig
	PID                 int
	LeaveCallCount      int
	UseKeyCallCount     int
	InstallKeyCallCount int
	StatsCallCount      int
}

func NewOutputWriter(path string, pid int, args []string, configDir string) *OutputWriter {
	consulConfig, err := decodeConsulConfig(filepath.Join(configDir, "config.json"))
	if err != nil {
		panic(err)
	}

	ow := &OutputWriter{
		filepath: path,
		data: OutputData{
			PID:          pid,
			Args:         args,
			ConsulConfig: consulConfig,
		},
		callCountChan: make(chan string),
	}

	go ow.run()

	return ow
}

func decodeConsulConfig(path string) (ConsulConfig, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return ConsulConfig{}, err
	}

	var consulConfig ConsulConfig
	err = json.Unmarshal(contents, &consulConfig)
	if err != nil {
		return ConsulConfig{}, err
	}

	return consulConfig, nil
}

func (ow *OutputWriter) run() {
	ow.writeOutput()
	for {
		switch <-ow.callCountChan {
		case "leave":
			ow.data.LeaveCallCount++
		case "installkey":
			ow.data.InstallKeyCallCount++
		case "usekey":
			ow.data.UseKeyCallCount++
		case "stats":
			ow.data.StatsCallCount++
		case "exit":
			return
		}
		ow.writeOutput()
	}
}

func (ow OutputWriter) writeOutput() {
	outputBytes, err := json.Marshal(ow.data)
	if err != nil {
		panic(err)
	}

	// save information JSON to the config dir
	err = ioutil.WriteFile(ow.filepath, outputBytes, 0600)
	if err != nil {
		panic(err)
	}
}

func (ow *OutputWriter) LeaveCalled() {
	ow.callCountChan <- "leave"
}

func (ow *OutputWriter) UseKeyCalled() {
	ow.callCountChan <- "usekey"
}

func (ow *OutputWriter) InstallKeyCalled() {
	ow.callCountChan <- "installkey"
}

func (ow *OutputWriter) StatsCalled() {
	ow.callCountChan <- "stats"
}

func (ow *OutputWriter) Exit() {
	ow.callCountChan <- "exit"
}
