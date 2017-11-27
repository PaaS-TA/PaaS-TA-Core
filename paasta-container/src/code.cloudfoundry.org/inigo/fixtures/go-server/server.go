package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/env", env)
	http.HandleFunc("/write", write)
	http.HandleFunc("/curl", curl)
	http.HandleFunc("/yo", yo)
	http.HandleFunc("/privileged", privileged)
	http.HandleFunc("/cf-instance-cert", cfInstanceCert)
	http.HandleFunc("/cf-instance-key", cfInstanceKey)

	fmt.Println("listening...")

	ports := os.Getenv("PORT")
	httpsPort := os.Getenv("HTTPS_PORT")

	portArray := strings.Split(ports, " ")

	errCh := make(chan error)

	for _, port := range portArray {
		println(port)
		go func(port string) {
			errCh <- http.ListenAndServe(":"+port, nil)
		}(port)

	}

	if httpsPort != "" {
		go func() {
			instanceCertPath := os.Getenv("CF_INSTANCE_CERT")
			instanceKeyPath := os.Getenv("CF_INSTANCE_KEY")
			errCh <- http.ListenAndServeTLS(":"+httpsPort, instanceCertPath, instanceKeyPath, nil)
		}()
	}

	err := <-errCh
	if err != nil {
		panic(err)
	}
}

type VCAPApplication struct {
	InstanceIndex int `json:"instance_index"`
}

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(res, "%s", os.Getenv("INSTANCE_INDEX"))
}

func write(res http.ResponseWriter, req *http.Request) {
	mountPointPath := os.Getenv("MOUNT_POINT_DIR") + "/test.txt"

	d1 := []byte("Hello Persistant World!\n")
	err := ioutil.WriteFile(mountPointPath, d1, 0644)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}

	res.WriteHeader(http.StatusOK)
	body, err := ioutil.ReadFile(mountPointPath)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte(err.Error()))
		return
	}
	res.Write(body)
	return
}

func env(res http.ResponseWriter, req *http.Request) {
	for _, e := range os.Environ() {
		fmt.Fprintf(res, "%s\n", e)
	}
}

func curl(res http.ResponseWriter, req *http.Request) {
	cmd := exec.Command("curl", "--connect-timeout", "5", "http://www.example.com")
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			fmt.Fprint(res, "Unknown Exit Code\n")
		}

		waitStatus := exitErr.Sys().(syscall.WaitStatus)
		fmt.Fprintf(res, "%d", waitStatus.ExitStatus())
		return
	}

	fmt.Fprintf(res, "%d", 0)
}

func yo(res http.ResponseWriter, req *http.Request) {
	fmt.Fprint(res, "sup dawg")
}

func privileged(res http.ResponseWriter, req *http.Request) {
	cmd := exec.Command("touch", "/proc/sysrq-trigger")
	err := cmd.Run()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "Failed to touch file: %s\n", err.Error())
		return
	}

	fmt.Fprint(res, "Success\n")
}

func cfInstanceCert(res http.ResponseWriter, req *http.Request) {
	path := os.Getenv("CF_INSTANCE_CERT")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Write(data)
	return
}

func cfInstanceKey(res http.ResponseWriter, req *http.Request) {
	path := os.Getenv("CF_INSTANCE_KEY")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.Write(data)
	return
}
