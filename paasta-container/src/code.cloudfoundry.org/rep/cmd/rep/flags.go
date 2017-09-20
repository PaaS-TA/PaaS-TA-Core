package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep"
)

var sessionName = flag.String(
	"sessionName",
	"rep",
	"consul session name",
)

var consulCluster = flag.String(
	"consulCluster",
	"",
	"comma-separated list of consul server URLs (scheme://ip:port)",
)

var lockTTL = flag.Duration(
	"lockTTL",
	locket.DefaultSessionTTL,
	"TTL for service lock",
)

var lockRetryInterval = flag.Duration(
	"lockRetryInterval",
	locket.RetryInterval,
	"interval to wait before retrying a failed lock acquisition",
)

var listenAddr = flag.String(
	"listenAddr",
	"0.0.0.0:1800",
	"host:port to serve auction and LRP stop requests on",
)

var listenAddrSecurable = flag.String(
	"listenAddrSecurable",
	"0.0.0.0:1801",
	"host:port to serve auction and LRP stop requests on",
)

var requireTLS = flag.Bool(
	"requireTLS",
	true,
	"Whether to require mutual TLS for communication to the securable rep API server",
)

var caFile = flag.String(
	"caFile",
	"",
	"the certificate authority public key file to use with tls authentication",
)

var certFile = flag.String(
	"certFile",
	"",
	"the public key file to use with tls authentication",
)

var keyFile = flag.String(
	"keyFile",
	"",
	"the private key file to use with ssl authentication",
)

var cellID = flag.String(
	"cellID",
	"",
	"the ID used by the rep to identify itself to external systems - must be specified",
)

var zone = flag.String(
	"zone",
	"",
	"the availability zone associated with the rep",
)

var pollingInterval = flag.Duration(
	"pollingInterval",
	30*time.Second,
	"the interval on which to scan the executor",
)

var dropsondePort = flag.Int(
	"dropsondePort",
	3457,
	"port the local metron agent is listening on",
)

var communicationTimeout = flag.Duration(
	"communicationTimeout",
	10*time.Second,
	"Timeout applied to all HTTP requests.",
)

var evacuationTimeout = flag.Duration(
	"evacuationTimeout",
	10*time.Minute,
	"Timeout to wait for evacuation to complete",
)

var evacuationPollingInterval = flag.Duration(
	"evacuationPollingInterval",
	10*time.Second,
	"the interval on which to scan the executor during evacuation",
)

var bbsAddress = flag.String(
	"bbsAddress",
	"",
	"Address to the BBS Server",
)

var advertiseDomain = flag.String(
	"advertiseDomain",
	"cell.service.cf.internal",
	"base domain at which the rep advertises its secure domain api",
)

var enableLegacyApiServer = flag.Bool(
	"enableLegacyApiServer",
	true,
	"Whether to enable the auction, LRP, and Task endpoints on the legacy, insecurable API server",
)

var bbsCACert = flag.String(
	"bbsCACert",
	"",
	"path to certificate authority cert used for mutually authenticated TLS BBS communication",
)

var bbsClientCert = flag.String(
	"bbsClientCert",
	"",
	"path to client cert used for mutually authenticated TLS BBS communication",
)

var bbsClientKey = flag.String(
	"bbsClientKey",
	"",
	"path to client key used for mutually authenticated TLS BBS communication",
)

var bbsClientSessionCacheSize = flag.Int(
	"bbsClientSessionCacheSize",
	0,
	"Capacity of the ClientSessionCache option on the TLS configuration. If zero, golang's default will be used",
)

var bbsMaxIdleConnsPerHost = flag.Int(
	"bbsMaxIdleConnsPerHost",
	0,
	"Controls the maximum number of idle (keep-alive) connctions per host. If zero, golang's default will be used",
)

type stackPathMap rep.StackPathMap

func (s *stackPathMap) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stackPathMap) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return errors.New("Invalid preloaded RootFS value: not of the form 'stack-name:path'")
	}

	if parts[0] == "" {
		return errors.New("Invalid preloaded RootFS value: blank stack")
	}

	if parts[1] == "" {
		return errors.New("Invalid preloaded RootFS value: blank path")
	}

	(*s)[parts[0]] = parts[1]
	return nil
}

type multiArgList []string

func (p *multiArgList) String() string {
	return fmt.Sprintf("%v", *p)
}

func (p *multiArgList) Set(value string) error {
	if value == "" {
		return errors.New("Cannot set blank value")
	}

	*p = append(*p, value)
	return nil
}

type commaSeparatedArgList []string

func (a *commaSeparatedArgList) String() string {
	return fmt.Sprintf("%v", *a)
}

func (a *commaSeparatedArgList) Set(value string) error {
	*a = strings.Split(value, ",")
	return nil
}
