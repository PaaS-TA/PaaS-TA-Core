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

type outputData struct {
	Args []string
	PID  int
}

func main() {
	// store information about this fake process into JSON
	signal.Ignore()

	var data outputData
	data.PID = os.Getpid()
	data.Args = os.Args[1:]

	// validate command line arguments
	// expect them to look like
	//   fake-thing agent -config-dir=/some/path/to/some/dir
	if len(data.Args) == 0 {
		log.Fatal("expecting command as first argment")
	}

	var configDir string
	var recursors stringSlice
	flagSet := flag.NewFlagSet("", flag.ExitOnError)
	flagSet.StringVar(&configDir, "config-dir", "", "config directory")
	flagSet.Var(&recursors, "recursor", "recursor")
	flagSet.Parse(data.Args[1:])
	if configDir == "" {
		log.Fatal("missing required config-dir flag")
	}

	writeOutput(configDir, data)

	// read input options provided to us by the test
	var inputOptions struct {
		WaitForHUP bool
	}

	if optionsBytes, err := ioutil.ReadFile(filepath.Join(configDir, "options.json")); err == nil {
		json.Unmarshal(optionsBytes, &inputOptions)
	}

	fmt.Fprintf(os.Stdout, "some standard out")
	fmt.Fprintf(os.Stderr, "some standard error")

	if inputOptions.WaitForHUP {
		for {
			time.Sleep(time.Second)
		}
	}
}

func writeOutput(configDir string, data outputData) {
	outputBytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	// save information JSON to the config dir
	err = ioutil.WriteFile(filepath.Join(configDir, "fake-output.json"), outputBytes, 0600)
	if err != nil {
		panic(err)
	}
}
