package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

type stringSlice []string

func (ss *stringSlice) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *stringSlice) Set(value string) error {
	*ss = append(*ss, value)

	return nil
}

func main() {
	// store information about this fake process into JSON
	signal.Ignore()

	// validate command line arguments
	// expect them to look like
	//   fake-thing agent -config-dir=/some/path/to/some/dir
	if len(os.Args[1:]) == 0 {
		log.Fatal("expecting command as first argment")
	}

	var configDir string
	var recursors stringSlice
	flagSet := flag.NewFlagSet("", flag.ExitOnError)
	flagSet.StringVar(&configDir, "config-dir", "", "config directory")
	flagSet.Var(&recursors, "recursor", "recursor")
	flagSet.Parse(os.Args[2:])
	if configDir == "" {
		log.Fatal("missing required config-dir flag")
	}

	// read input options provided to us by the test
	var inputOptions struct {
		Members           []string
		FailStatsEndpoint bool
	}

	if optionsBytes, err := ioutil.ReadFile(filepath.Join(configDir, "options.json")); err == nil {
		json.Unmarshal(optionsBytes, &inputOptions)
	}

	outputFile := filepath.Join(configDir, "fake-output.json")
	if _, err := os.Stat(outputFile); err == nil {
		outputFile = filepath.Join(configDir, "fake-output-2.json")
	}
	ow := NewOutputWriter(outputFile, os.Getpid(), os.Args[1:], configDir)

	server := &Server{
		HTTPAddr:          "127.0.0.1:8500",
		Members:           inputOptions.Members,
		OutputWriter:      ow,
		FailStatsEndpoint: inputOptions.FailStatsEndpoint,
	}

	err := server.Serve()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n", err)
	}

	for {
		if server.DidLeave {
			err := server.Exit()
			if err != nil {
				log.Fatalf("Failed to close server: %s\n", err)
			}

			ow.Exit()
			os.Exit(0)
		}

		time.Sleep(100 * time.Millisecond)
	}
}
