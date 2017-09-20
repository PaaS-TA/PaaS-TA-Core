package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
)

type DropHandler struct {
	logger   *log.Logger
	ipTables IPTablesWrapper
}

type IPTablesWrapper interface {
	Run([]string) (string, error)
}

func NewDropHandler(ipTablesWrapper IPTablesWrapper, writer io.Writer) DropHandler {
	return DropHandler{
		logger:   log.New(writer, "[drop handler] ", log.LUTC),
		ipTables: ipTablesWrapper,
	}
}

type RealIPTables struct {
	command string
}

func NewIPTables(command string) IPTablesWrapper {
	return RealIPTables{command: command}
}

func (iptables RealIPTables) Run(args []string) (string, error) {
	cmd := exec.Command(iptables.command, args...)
	output := bytes.NewBuffer([]byte{})
	cmd.Stdout = output
	cmd.Stderr = output
	err := cmd.Run()
	if err != nil {
		return output.String(), err
	}

	return "", nil
}

func (d *DropHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	d.logger.Printf("received request: %s %s\n", req.Method, req.URL.String())
	if req.Method != "PUT" && req.Method != "DELETE" {
		d.logger.Println("error: not a PUT or DELETE request")
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	addr, err := d.queryParams(req.URL)
	if err != nil {
		d.logger.Printf("error: missing required params (%s)\n", err.Error())
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte(err.Error()))
		return
	}

	command := "-A"
	if req.Method == "DELETE" {
		command = "-D"
	}

	output, err := d.ipTables.Run([]string{command, "INPUT", "-s", addr, "-j", "DROP"})
	if err != nil {
		d.logger.Printf("error: iptables failed (%s)\n", err)
		d.logger.Printf("error: iptables failed output: %s\n", string(output))
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(fmt.Sprintf("error: %s\niptables output: %s", err.Error(), string(output))))
		return
	}

	output, err = d.ipTables.Run([]string{command, "OUTPUT", "-d", addr, "-j", "DROP"})
	if err != nil {
		d.logger.Printf("error: iptables failed (%s)\n", err)
		d.logger.Printf("error: iptables failed output: %s\n", string(output))
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(fmt.Sprintf("error: %s\niptables output: %s", err.Error(), string(output))))
		return
	}

	d.logger.Println("request successful")
}

func (d *DropHandler) queryParams(url *url.URL) (string, error) {
	queryVals := url.Query()
	addr := queryVals.Get("addr")
	if addr == "" {
		return "", errors.New("must provide addr param")
	}

	return addr, nil
}
