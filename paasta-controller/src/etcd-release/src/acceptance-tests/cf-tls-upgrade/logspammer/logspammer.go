package logspammer

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"

	"github.com/cloudfoundry/sonde-go/events"
)

// Use a custom time format to avoid losing trailing 0s in the nanoseconds.
const TimeFormat = "2006-01-02 15:04:05.000000000 -0700 MST"

var (
	timeNow (func() time.Time)
)

type Spammer struct {
	sync.Mutex
	appURL          string
	frequency       time.Duration
	doneGet         chan struct{}
	doneMsg         chan struct{}
	wg              sync.WaitGroup
	doneWaitGroup   sync.WaitGroup
	logMessages     []string
	logWritten      int
	logsWritten     map[int]time.Time
	msgChan         <-chan *events.Envelope
	errChan         <-chan error
	errors          helpers.ErrorSet
	prefix          string
	logger          io.Writer
	streamGenerator func() (<-chan *events.Envelope, <-chan error)
}

func NewSpammer(logger io.Writer, appURL string, streamGenerator func() (<-chan *events.Envelope, <-chan error), frequency time.Duration) *Spammer {
	timeNow = time.Now

	msgChan, errChan := streamGenerator()
	return &Spammer{
		appURL:          appURL,
		frequency:       frequency,
		doneGet:         make(chan struct{}),
		doneMsg:         make(chan struct{}),
		msgChan:         msgChan,
		errChan:         errChan,
		errors:          helpers.ErrorSet{},
		prefix:          fmt.Sprintf("spammer-%d", rand.Int()),
		logMessages:     []string{},
		logger:          logger,
		streamGenerator: streamGenerator,
		logsWritten:     map[int]time.Time{},
	}
}

func (s *Spammer) CheckStream() bool {
	resp, err := http.Get(fmt.Sprintf("%s/log/TEST", s.appURL))
	if err != nil {
		return false
	}

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	err = resp.Body.Close()
	if err != nil {
		return false
	}

	select {
	case <-s.msgChan:
		return true
	case <-time.After(5 * time.Millisecond):
		return false
	}
}

func (s *Spammer) Start() error {
	s.wg.Add(1)
	s.doneWaitGroup.Add(1)
	go func() {
		for {
			select {
			case <-s.doneGet:
				s.wg.Done()
				close(s.doneMsg)
				s.doneWaitGroup.Wait()
				return
			case <-time.After(s.frequency):
				resp, err := http.Get(fmt.Sprintf("%s/log/%s-%d-", s.appURL, s.prefix, s.logWritten))
				if err != nil {
					s.errors.Add(err)
					continue
				}

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					s.errors.Add(err)
					continue
				}

				err = resp.Body.Close()
				if err != nil {
					s.errors.Add(err)
					continue
				}

				if resp.StatusCode != http.StatusOK {
					s.errors.Add(fmt.Errorf("%+v -- %v", resp, string(body)))
					continue
				}

				s.logsWritten[s.logWritten] = timeNow()
				s.logWritten++
			}
		}
	}()

	go func() {
		for {
			select {
			case <-s.doneMsg:
				return
			case err := <-s.errChan:
				if strings.Contains(err.Error(), "Unauthorized") {
					s.msgChan, s.errChan = s.streamGenerator()
				}
				continue
			case msg := <-s.msgChan:
				s.Lock()
				if msg != nil {
					if msg.LogMessage != nil {
						if *msg.LogMessage.SourceType == "APP/PROC/WEB" && *msg.LogMessage.MessageType == events.LogMessage_OUT {
							s.logMessages = append(s.logMessages, string(msg.LogMessage.Message))
						}
					}
				}
				s.Unlock()
			}
		}
	}()

	return nil
}

func (s *Spammer) Stop() error {
	close(s.doneGet)
	s.wg.Wait()
	return nil
}

func (s *Spammer) Check() (error, error) {
	receivedLogNumbers := s.getReceivedLogNumbers()
	logMissing := helpers.ErrorSet{}

	missingLogs := []time.Time{}
	for expectedLogNumber := 0; expectedLogNumber < s.logWritten; expectedLogNumber++ {
		if _, found := receivedLogNumbers[expectedLogNumber]; !found {
			missingLogs = append(missingLogs, s.logsWritten[expectedLogNumber])
		}
	}

	if len(missingLogs) > 0 {
		s.logger.Write([]byte(fmt.Sprintf("total logs written %d, received logs %d, diff %d\n",
			s.logWritten,
			len(receivedLogNumbers),
			s.logWritten-len(receivedLogNumbers))))

		for _, log := range missingLogs {
			logMissing.Add(fmt.Errorf("missing log entry: %s", log.Format(TimeFormat)))
		}
	}

	if len(s.errors) > 0 && len(logMissing) > 0 {
		return s.errors, logMissing
	} else if len(s.errors) > 0 {
		return s.errors, nil
	} else if len(logMissing) > 0 {
		return nil, logMissing
	}
	return nil, nil
}

func (s *Spammer) getReceivedLogNumbers() map[int]bool {
	receivedMessages := s.LogMessages()

	receivedLogNumbers := map[int]bool{}
	dropTimestamp := regexp.MustCompile(`\[.*\] `)
	for _, logMessage := range receivedMessages {
		// We ignore the message that has the word TEST
		if strings.Contains(logMessage, "TEST") {
			continue
		}

		// [2016-10-12 17:47:49.268214492 +0000 UTC] spammer-324517860517642426-27126-
		log := dropTimestamp.ReplaceAllLiteralString(logMessage, "")
		if len(strings.Split(log, "-")) >= 3 {
			msgIntStr := strings.Split(log, "-")[2]
			msgInt, err := strconv.Atoi(msgIntStr)
			if err == nil {
				receivedLogNumbers[msgInt] = true
			}
		}
	}

	return receivedLogNumbers
}

func (s *Spammer) LogMessages() []string {
	s.Lock()
	defer s.Unlock()

	return s.logMessages
}
