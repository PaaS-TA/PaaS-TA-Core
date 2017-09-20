package buffered

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type ResponseWriter struct {
	status         int
	buffer         *bytes.Buffer
	header         http.Header
	responseWriter http.ResponseWriter
	logBuffer      *bytes.Buffer
}

func NewResponseWriter(w http.ResponseWriter, logBuffer *bytes.Buffer) *ResponseWriter {
	return &ResponseWriter{
		status:         http.StatusOK,
		buffer:         bytes.NewBuffer([]byte{}),
		header:         make(http.Header),
		responseWriter: w,
		logBuffer:      logBuffer,
	}
}

func (w ResponseWriter) Header() http.Header {
	return w.header
}

func (w ResponseWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *ResponseWriter) Copy() {
	contents := w.buffer.Bytes()

	if w.status == http.StatusInternalServerError {
		// we don't care about errors in this situation
		errorMessage, _ := w.logBuffer.ReadString(byte('\n'))

		contents = []byte(fmt.Sprintf("%s %s", strings.TrimSuffix(errorMessage, "\n"), contents))
	}

	for key, values := range w.Header() {
		for _, value := range values {
			w.responseWriter.Header().Add(key, value)
		}
	}

	w.responseWriter.Header().Set("Content-Length", strconv.Itoa(len(contents)))
	w.responseWriter.WriteHeader(w.status)
	w.responseWriter.Write(contents)
}
