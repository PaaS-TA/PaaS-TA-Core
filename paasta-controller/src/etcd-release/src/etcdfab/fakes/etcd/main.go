package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

var (
	backendURL string
)

func main() {
	fmt.Println("starting fake etcd")
	fmt.Fprintln(os.Stderr, "fake error in stderr")

	bodyContents, err := json.Marshal(os.Args[1:])
	if err != nil {
		panic(err)
	}
	var bodyBuffer bytes.Buffer
	bodyBuffer.Write(bodyContents)
	response, err := http.Post(fmt.Sprintf("%s/call", backendURL), "application/json", &bodyBuffer)
	if err != nil {
		fmt.Printf("%s/call failed: %s", backendURL, err.Error())
	}

	if response.StatusCode != http.StatusOK {
		os.Exit(1)
	}

	fmt.Println("stopping fake etcd")
}
