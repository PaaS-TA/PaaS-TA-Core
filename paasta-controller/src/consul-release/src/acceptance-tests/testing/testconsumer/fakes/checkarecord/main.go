package main

import (
	"fmt"
	"os"
	"strings"
)

var (
	Addresses   string
	ServiceName string
)

func main() {
	if len(os.Args) != 2 {
		os.Exit(1)
	}

	if os.Args[1] == ServiceName {
		if Addresses == "" {
			os.Exit(1)
		}

		for _, address := range strings.Split(Addresses, ",") {
			fmt.Println(address)
		}
	}
}
