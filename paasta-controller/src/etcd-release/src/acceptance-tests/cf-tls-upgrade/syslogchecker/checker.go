package syslogchecker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/helpers"
)

const (
	totalErrorThreshold = 0.2
	maxErrsPerIteration = 2
)

type GuidGenerator interface {
	Generate() string
}

type Checker struct {
	syslogAppPrefix string
	guidGenerator   GuidGenerator
	errors          helpers.ErrorSet
	doneChn         chan struct{}
	retryAfter      time.Duration
	runner          cfRunner
	iterationCount  *int32
}

type cfRunner interface {
	Run(...string) ([]byte, error)
}

func New(syslogAppPrefix string, guidGenerator GuidGenerator, retryAfter time.Duration, runner cfRunner) Checker {
	return Checker{
		syslogAppPrefix: syslogAppPrefix,
		guidGenerator:   guidGenerator,
		errors:          helpers.ErrorSet{},
		doneChn:         make(chan struct{}),
		retryAfter:      retryAfter,
		runner:          runner,
		iterationCount:  new(int32),
	}
}

func (c Checker) Start(logSpinnerApp string, logSpinnerAppURL string) error {
	go func() {
		for {
			select {
			case <-c.doneChn:
				return
			case <-time.After(c.retryAfter):
				syslogDrainAppName := fmt.Sprintf("%s-%s", c.syslogAppPrefix, c.guidGenerator.Generate())
				if err := c.deploySyslogAndValidate(syslogDrainAppName, logSpinnerApp, logSpinnerAppURL); err != nil {
					c.errors.Add(err)
				}

				if err := c.cleanup(logSpinnerApp, syslogDrainAppName); err != nil {
					c.errors.Add(err)
				}
				atomic.AddInt32(c.iterationCount, 1)
			}
		}
	}()

	return nil
}

func (c Checker) Stop() error {
	c.doneChn <- struct{}{}
	return nil
}

func (c Checker) GetIterationCount() int32 {
	return *c.iterationCount
}

func (c Checker) deploySyslogAndValidate(syslogDrainAppName, logSpinnerApp, logSpinnerAppURL string) error {
	err := c.setupSyslogDrainerApp(syslogDrainAppName)
	if err != nil {
		return err
	}

	output, err := c.runner.Run("logs", syslogDrainAppName, "--recent")
	if err != nil {
		return fmt.Errorf("could not retrieve the logs from syslog-drainer app: %s", output)
	}

	address, err := getSyslogAddress(output)
	if err != nil {
		return err
	}

	output, err = c.runner.Run("cups", fmt.Sprintf("%s-service", syslogDrainAppName), "-l", fmt.Sprintf("syslog://%s", address))
	if err != nil {
		return fmt.Errorf("could not create the logger service: %s", output)
	}

	output, err = c.runner.Run("bind-service", logSpinnerApp, fmt.Sprintf("%s-service", syslogDrainAppName))
	if err != nil {
		return fmt.Errorf("could not bind the logger to the application: %s", output)
	}

	output, err = c.runner.Run("restage", logSpinnerApp)
	if err != nil {
		return fmt.Errorf("could not restage the app: %s", output)
	}

	guid := c.guidGenerator.Generate()
	if err := sendGetRequestToApp(fmt.Sprintf("%s/log/%s", logSpinnerAppURL, guid)); err != nil {
		return err
	}

	output, err = c.runner.Run("logs", syslogDrainAppName, "--recent")
	if err != nil {
		return fmt.Errorf("could not get the logs for syslog drainer app: %s", output)
	}

	if err := validateDrainerGotGuid(string(output), guid); err != nil {
		return err
	}

	return nil
}

func (c Checker) cleanup(logSpinnerApp, appName string) error {
	errs := make(helpers.ErrorSet)
	output, err := c.runner.Run("unbind-service", logSpinnerApp, fmt.Sprintf("%s-service", appName))
	if err != nil {
		errs.Add(fmt.Errorf("could not unbind the logger from the application: %s", output))
	}

	output, err = c.runner.Run("delete-service", fmt.Sprintf("%s-service", appName), "-f")
	if err != nil {
		errs.Add(fmt.Errorf("could not delete the service: %s", output))
	}

	output, err = c.runner.Run("delete", appName, "-f", "-r")
	if err != nil {
		errs.Add(fmt.Errorf("could not delete the syslog drainer app: %s", output))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (c Checker) setupSyslogDrainerApp(syslogDrainerAppName string) error {
	output, err := c.runner.Run("push", syslogDrainerAppName,
		"-f", filepath.Join(os.Getenv("GOPATH"), "src/github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/cf-tls-upgrade/syslogchecker/assets/manifest.yml"),
		"--no-start")
	if err != nil {
		return fmt.Errorf("syslog drainer application push failed: %s", output)
	}

	output, err = c.runner.Run("app", syslogDrainerAppName, "--guid")
	if err != nil {
		return fmt.Errorf("failed to get the guid for the app %q: %s", syslogDrainerAppName, output)
	}

	output, err = c.runner.Run("curl", fmt.Sprintf("/v2/apps/%s", output), "-X", "PUT", "-d", `{"diego": true}`)
	if err != nil {
		return fmt.Errorf("failed to get enable diego for the app %q: %s", syslogDrainerAppName, output)
	}

	output, err = c.runner.Run("start", syslogDrainerAppName)
	if err != nil {
		return fmt.Errorf("could not start the syslog-drainer app: %s", output)
	}
	return nil
}

func (c Checker) Check() (bool, int, float64, error) {
	iterationCount := int(c.GetIterationCount())
	errsOccurred := 0
	for _, errCount := range c.errors {
		errsOccurred += errCount
	}
	errPercent := float64(errsOccurred) / (maxErrsPerIteration * float64(iterationCount))
	ok := true
	if errPercent > totalErrorThreshold {
		ok = false
		c.errors.Add(fmt.Errorf("ran %v times, exceeded total error threshold of %v: %v", iterationCount, totalErrorThreshold, errPercent))
	}

	return ok, iterationCount, errPercent, c.errors
}

func getSyslogAddress(output []byte) (string, error) {
	re, err := regexp.Compile("ADDRESS: \\|([0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\:[0-9]+)\\|")
	if err != nil {
		return "", err
	}

	matches := re.FindSubmatch(output)
	if len(matches) < 2 {
		return "", errors.New("could not parse the IP address of syslog-drain app")
	}

	return string(matches[1]), nil
}

func sendGetRequestToApp(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error sending get request to listener app: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

func validateDrainerGotGuid(logContents string, guid string) error {
	var validLogLine = regexp.MustCompile(fmt.Sprintf("\\[APP.*\\].*%s", guid))
	if !validLogLine.MatchString(logContents) {
		return errors.New("could not validate the guid on syslog")
	}
	return nil
}
