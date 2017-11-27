package testrunner

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/cmd/locket/config"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var (
	fixturesPath = filepath.Join(os.Getenv("GOPATH"), "src/code.cloudfoundry.org/locket/cmd/locket/fixtures")

	caCertFile = filepath.Join(fixturesPath, "ca.crt")
	certFile   = filepath.Join(fixturesPath, "cert.crt")
	keyFile    = filepath.Join(fixturesPath, "key.key")
)

func NewLocketRunner(locketBinPath string, fs ...func(cfg *config.LocketConfig)) ifrit.Runner {
	cfg := &config.LocketConfig{
		CaFile:   caCertFile,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	for _, f := range fs {
		f(cfg)
	}

	locketConfig, err := ioutil.TempFile("", "locket-config")
	Expect(err).NotTo(HaveOccurred())

	locketConfigFilePath := locketConfig.Name()

	encoder := json.NewEncoder(locketConfig)
	err = encoder.Encode(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(locketConfig.Close()).To(Succeed())

	return ginkgomon.New(ginkgomon.Config{
		Name:              "locket",
		StartCheck:        "locket.started",
		StartCheckTimeout: 10 * time.Second,
		Command:           exec.Command(locketBinPath, "-config="+locketConfigFilePath),
		Cleanup: func() {
			os.RemoveAll(locketConfigFilePath)
		},
	})
}

func LocketClientTLSConfig() *tls.Config {
	tlsConfig, err := cfhttp.NewTLSConfig(certFile, keyFile, caCertFile)
	Expect(err).NotTo(HaveOccurred())
	return tlsConfig
}

func ClientLocketConfig() locket.ClientLocketConfig {
	return locket.ClientLocketConfig{
		LocketCACertFile:     caCertFile,
		LocketClientCertFile: certFile,
		LocketClientKeyFile:  keyFile,
	}
}
