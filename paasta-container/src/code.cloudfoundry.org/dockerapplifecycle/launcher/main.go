package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/dockerapplifecycle/protocol"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <ignored> <start command> <metadata>", os.Args[0])
		os.Exit(1)
	}

	// os.Args[1] is ignored, but left for backwards compatibility
	startCommand := os.Args[2]
	metadata := os.Args[3]

	vcapAppEnv := map[string]interface{}{}

	err := json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), &vcapAppEnv)
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

	var executionMetadata protocol.ExecutionMetadata
	err = json.Unmarshal([]byte(metadata), &executionMetadata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid metadata - %s", err)
		os.Exit(1)
	}

	workdir := "/"
	if executionMetadata.Workdir != "" {
		workdir = executionMetadata.Workdir
	}
	os.Chdir(workdir)

	if len(executionMetadata.Entrypoint) == 0 && len(executionMetadata.Cmd) == 0 && startCommand == "" {
		fmt.Fprintf(os.Stderr, "No start command found or specified")
		os.Exit(1)
	}

	// https://docs.docker.com/reference/builder/#entrypoint and
	// https://docs.docker.com/reference/builder/#cmd dictate how Entrypoint
	// and Cmd are treated by docker; we follow these rules here
	var argv []string
	if startCommand != "" {
		argv = []string{"/bin/sh", "-c", startCommand}
	} else {
		argv = append(executionMetadata.Entrypoint, executionMetadata.Cmd...)
		argv[0], err = exec.LookPath(argv[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve path: %s", err)
			os.Exit(1)
		}
	}

	runtime.GOMAXPROCS(1)
	err = syscall.Exec(argv[0], argv, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run: %s", err)
		os.Exit(1)
	}
}
