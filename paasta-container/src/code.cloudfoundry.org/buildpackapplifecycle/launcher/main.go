package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	yaml "gopkg.in/yaml.v2"
)

func main() {
	if len(os.Args) < 4 {
		exitWithUsage()
	}

	dir := os.Args[1]
	startCommand := os.Args[2]

	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}
	os.Setenv("HOME", dir)

	tmpDir, err := filepath.Abs(filepath.Join(dir, "..", "tmp"))
	if err == nil {
		os.Setenv("TMPDIR", tmpDir)
	}

	depsDir, err := filepath.Abs(filepath.Join(dir, "..", "deps"))
	if err == nil {
		os.Setenv("DEPS_DIR", depsDir)
	}

	vcapAppEnv := map[string]interface{}{}
	err = json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), &vcapAppEnv)
	if err == nil {
		vcapAppEnv["host"] = "0.0.0.0"

		vcapAppEnv["instance_id"] = os.Getenv("INSTANCE_GUID")

		port, err := strconv.Atoi(os.Getenv("PORT"))
		if err == nil {
			vcapAppEnv["port"] = port
		}

		index, err := strconv.Atoi(os.Getenv("INSTANCE_INDEX"))
		if err == nil {
			vcapAppEnv["instance_index"] = index
		}

		mungedAppEnv, err := json.Marshal(vcapAppEnv)
		if err == nil {
			os.Setenv("VCAP_APPLICATION", string(mungedAppEnv))
		}
	}

	var command string
	if startCommand != "" {
		command = startCommand
	} else {
		command, err = startCommandFromStagingInfo("staging_info.yml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid staging info - %s", err)
			os.Exit(1)
		}
	}

	if command == "" {
		exitWithUsage()
	}

	runtime.GOMAXPROCS(1)
	runProcess(dir, command)
}

func exitWithUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <app directory> <start command> <metadata>", os.Args[0])
	os.Exit(1)
}

type stagingInfo struct {
	StartCommand string `yaml:"start_command"`
}

func startCommandFromStagingInfo(stagingInfoPath string) (string, error) {
	stagingInfoData, err := ioutil.ReadFile(stagingInfoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	info := stagingInfo{}

	err = yaml.Unmarshal(stagingInfoData, &info)
	if err != nil {
		return "", errors.New("invalid YAML")
	}

	return info.StartCommand, nil
}
