package handlers

import (
	"fmt"
	"net/http"
)

type MonitHandler struct {
	command      string
	monitWrapper monitWrapper
	logger       logger
	removeStore  removeStore
}

type monitWrapper interface {
	Run(args []string) error
	Output(args []string) (string, error)
}

type removeStore interface {
	DeleteContents(storeDir string) error
}

type logger interface {
	Println(v ...interface{})
}

func NewMonitHandler(command string, monitWrapper monitWrapper, removeStore removeStore, logger logger) MonitHandler {
	return MonitHandler{
		command:      command,
		monitWrapper: monitWrapper,
		removeStore:  removeStore,
		logger:       logger,
	}
}

func (m MonitHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	queryVals := req.URL.Query()
	job := queryVals.Get("job")
	deleteStore := queryVals.Get("delete_store") == "true"

	m.logger.Println(fmt.Sprintf("[INFO] monit %s %s", m.command, job))
	err := m.monitWrapper.Run([]string{
		m.command,
		job,
	})
	if err != nil {
		m.logger.Println(fmt.Sprintf("[ERR] %s", err.Error()))
	}

	if deleteStore {
		m.logger.Println(fmt.Sprintf("[INFO] deleting /var/vcap/store/%s contents", job))
		err = m.removeStore.DeleteContents(fmt.Sprintf("/var/vcap/store/%s", job))
		if err != nil {
			m.logger.Println(fmt.Sprintf("[ERR] %s", err.Error()))
		}
	}
}
