package log_streamer

import (
	"io"
	"io/ioutil"
)

type noopStreamer struct{}

func NewNoopStreamer() LogStreamer {
	return noopStreamer{}
}

func (noopStreamer) Stdout() io.Writer { return ioutil.Discard }
func (noopStreamer) Stderr() io.Writer { return ioutil.Discard }
func (noopStreamer) Flush()            {}
func (noopStreamer) WithSource(sourceName string) LogStreamer {
	return noopStreamer{}
}
