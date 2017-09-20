package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

type DNSHandler struct {
	pathToCheckARecord string
}

func NewDNSHandler(pathToCheckARecord string) DNSHandler {
	return DNSHandler{
		pathToCheckARecord: pathToCheckARecord,
	}
}

func (d DNSHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	serviceName := request.URL.Query().Get("service")

	if serviceName == "" {
		response.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(response, "service is a required parameter")
		return
	}

	var addresses []string
	defer func() {
		log.Println("Querying DNS for ", serviceName, " results being ", addresses)
	}()

	command := exec.Command(d.pathToCheckARecord, serviceName)

	stdout, err := command.Output()
	switch err.(type) {
	case nil:
		addresses = strings.Split(strings.TrimSpace(string(stdout)), "\n")
	case *exec.ExitError:
		log.Println("Some exit error occured : ", err)
		addresses = []string{}
	default:
		log.Println("Some error occured : ", err)
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, err.Error())
		return
	}

	buf, err := json.Marshal(addresses)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(response, err.Error())
		return
	}

	fmt.Fprint(response, string(buf))
}
