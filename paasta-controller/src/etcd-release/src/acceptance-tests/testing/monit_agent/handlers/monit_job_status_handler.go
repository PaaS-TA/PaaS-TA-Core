package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

type MonitJobStatusHandler struct {
	monitWrapper monitWrapper
	logger       logger
}

func NewMonitJobStatusHandler(monitWrapper monitWrapper, logger logger) MonitJobStatusHandler {
	return MonitJobStatusHandler{
		monitWrapper: monitWrapper,
		logger:       logger,
	}
}

func (m MonitJobStatusHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	queryVals := req.URL.Query()
	job := queryVals.Get("job")

	m.logger.Println(fmt.Sprintf("[INFO] get job status of %s using monit summary", job))
	output, err := m.monitWrapper.Output([]string{"summary"})
	if err != nil {
		m.logger.Println(fmt.Sprintf("[ERR] %s", err.Error()))
	}
	m.logger.Println(fmt.Sprintf("[INFO] monit summary output: %s", output))

	status := getJobStatusFromMonitSummaryOutput(job, output)
	_, err = w.Write([]byte(status))
	if err != nil {
		// Not Tested
		m.logger.Println(fmt.Sprintf("[ERR] %s", err.Error()))
	}
}

func getJobStatusFromMonitSummaryOutput(job, output string) string {
	lines := strings.Split(output, "\n")
	var jobLine string
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf("Process '%s'", job)) {
			jobLine = line
		}
	}

	tokens := strings.Split(jobLine, "'")
	jobStatus := strings.TrimSpace(tokens[len(tokens)-1])

	return jobStatus
}
