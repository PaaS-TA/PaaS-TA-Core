package main_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/consuladapter/consulrunner"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/routing-api"
	routingtestrunner "code.cloudfoundry.org/routing-api/cmd/routing-api/testrunner"
	"code.cloudfoundry.org/tcp-emitter/cmd/tcp-emitter/testrunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

var (
	routingAPIBinPath string
	routingAPIAddress string
	routingAPIArgs    routingtestrunner.Args
	routingAPIPort    uint16
	routingAPIIP      string
	routingApiClient  routing_api.Client

	oauthServer     *ghttp.Server
	oauthServerPort string

	dbAllocator routingtestrunner.DbAllocator
	dbId        string

	bbsServer *ghttp.Server
	bbsPort   string

	tcpEmitterBinPath string
	tcpEmitterArgs    testrunner.Args

	consulRunner *consulrunner.ClusterRunner

	logger lager.Logger
)

func TestTcpEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TcpEmitter Suite")
}

func setupDB() {
	dbAllocator = routingtestrunner.NewDbAllocator(4001 + GinkgoParallelNode())

	var err error
	dbId, err = dbAllocator.Create()
	Expect(err).NotTo(HaveOccurred())
}

var _ = SynchronizedBeforeSuite(func() []byte {
	routingAPIBin, err := gexec.Build("code.cloudfoundry.org/routing-api/cmd/routing-api", "-race")
	Expect(err).NotTo(HaveOccurred())

	tcpEmitterBin, err := gexec.Build("code.cloudfoundry.org/tcp-emitter/cmd/tcp-emitter", "-race")
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"routing-api": routingAPIBin,
		"tcp-emitter": tcpEmitterBin,
	})

	Expect(err).NotTo(HaveOccurred())
	return payload
}, func(payload []byte) {
	logger = lagertest.NewTestLogger("test")
	context := map[string]string{}

	err := json.Unmarshal(payload, &context)
	Expect(err).NotTo(HaveOccurred())

	routingAPIBinPath = context["routing-api"]
	tcpEmitterBinPath = context["tcp-emitter"]

	setupDB()

	oauthServer = ghttp.NewUnstartedServer()
	basePath, err := filepath.Abs(path.Join("..", "..", "fixtures", "certs"))
	Expect(err).ToNot(HaveOccurred())
	cert, err := tls.LoadX509KeyPair(filepath.Join(basePath, "server.pem"), filepath.Join(basePath, "server.key"))
	Expect(err).ToNot(HaveOccurred())

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	oauthServer.HTTPTestServer.TLS = tlsConfig
	oauthServer.AllowUnhandledRequests = true
	oauthServer.UnhandledRequestStatusCode = http.StatusOK

	oauthServer.HTTPTestServer.StartTLS()
	oauthServerPort = getServerPort(oauthServer.URL())

	publicKey := "-----BEGIN PUBLIC KEY-----\\n" +
		"MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDHFr+KICms+tuT1OXJwhCUmR2d\\n" +
		"KVy7psa8xzElSyzqx7oJyfJ1JZyOzToj9T5SfTIq396agbHJWVfYphNahvZ/7uMX\\n" +
		"qHxf+ZH9BL1gk9Y6kCnbM5R60gfwjyW1/dQPjOzn9N394zd2FJoFHwdq9Qs0wBug\\n" +
		"spULZVNRxq7veq/fzwIDAQAB\\n" +
		"-----END PUBLIC KEY-----"

	data := fmt.Sprintf("{\"alg\":\"rsa\", \"value\":\"%s\"}", publicKey)
	oauthServer.RouteToHandler("GET", "/token_key",
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/token_key"),
			ghttp.RespondWith(http.StatusOK, data)),
	)
	oauthServer.RouteToHandler("POST", "/oauth/token",
		func(w http.ResponseWriter, req *http.Request) {
			jsonBytes := []byte(`{"access_token":"some-token", "expires_in":10}`)
			w.Write(jsonBytes)
		})

	consulRunner = consulrunner.NewClusterRunner(consulrunner.ClusterRunnerConfig{
		StartingPort: 9001 + config.GinkgoConfig.ParallelNode*consulrunner.PortOffsetLength,
		NumNodes:     1,
		Scheme:       "http",
	})

	logger.Info("started-oauth-server", lager.Data{"address": oauthServer.URL()})

})

var _ = BeforeEach(func() {
	var err error
	bbsServer = ghttp.NewServer()
	bbsServer.AllowUnhandledRequests = true
	bbsServer.UnhandledRequestStatusCode = http.StatusOK
	bbsPort = getServerPort(bbsServer.URL())
	logger.Info("started-bbs-server", lager.Data{"address": bbsServer.URL()})

	routingAPIPort = uint16(6900 + GinkgoParallelNode())
	routingAPIIP = "127.0.0.1"
	routingAPIAddress = fmt.Sprintf("http://%s:%d", routingAPIIP, routingAPIPort)

	routingAPIArgs, err = routingtestrunner.NewRoutingAPIArgs(
		routingAPIIP,
		routingAPIPort,
		dbId,
		consulRunner.URL())
	Expect(err).NotTo(HaveOccurred())

	routingApiClient = routing_api.NewClient(routingAPIAddress, false)

	tcpEmitterArgs = testrunner.Args{
		BBSAddress:     bbsServer.URL(),
		BBSClientCert:  createClientCert(),
		BBSCACert:      createCACert(),
		BBSClientKey:   createClientKey(),
		ConfigFilePath: createEmitterConfig(),
		SyncInterval:   1 * time.Second,
		ConsulCluster:  consulRunner.ConsulCluster(),
	}

	consulRunner.Start()
	consulRunner.WaitUntilReady()
})

var _ = AfterEach(func() {
	bbsServer.Close()
	consulRunner.Stop()
	if dbAllocator != nil {
		Expect(dbAllocator.Reset()).NotTo(HaveOccurred())
	}
})

var _ = SynchronizedAfterSuite(func() {
	if dbAllocator != nil {
		Expect(dbAllocator.Delete()).NotTo(HaveOccurred())
	}
	if oauthServer != nil {
		oauthServer.Close()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})

func getServerPort(url string) string {
	endpoints := strings.Split(url, ":")
	Expect(endpoints).To(HaveLen(3))
	return endpoints[2]
}

func createEmitterConfig(uaaPorts ...string) string {
	randomConfigFileName := fmt.Sprintf("tcp_router_%d.yml", GinkgoParallelNode())
	configFile := path.Join(os.TempDir(), randomConfigFileName)
	uaaPort := oauthServerPort
	if len(uaaPorts) > 0 {
		uaaPort = uaaPorts[0]
	}

	cfgString := `---
oauth:
  token_endpoint: "127.0.0.1"
  skip_ssl_validation: false
  ca_certs: %s
  client_name: "someclient"
  client_secret: "somesecret"
  port: %s
routing_api:
  uri: http://127.0.0.1
  port: %d
`
	caCertsPath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "certs", "uaa-ca.pem"))
	Expect(err).ToNot(HaveOccurred())
	cfg := fmt.Sprintf(cfgString, caCertsPath, uaaPort, routingAPIPort)

	err = writeToFile([]byte(cfg), configFile)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(fileExists(configFile)).To(BeTrue())
	return configFile
}

func createEmitterConfigAuthDisabled() string {
	randomConfigFileName := fmt.Sprintf("tcp_router_%d.yml", GinkgoParallelNode())
	configFile := path.Join(os.TempDir(), randomConfigFileName)

	cfgString := `---
oauth:
  token_endpoint: "127.0.0.1"
  skip_ssl_validation: true
  client_name: "someclient"
  client_secret: "somesecret"
  port: %s
routing_api:
  uri: http://127.0.0.1
  port: %d
  auth_disabled: true
`
	cfg := fmt.Sprintf(cfgString, oauthServerPort, routingAPIPort)

	err := writeToFile([]byte(cfg), configFile)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(fileExists(configFile)).To(BeTrue())
	return configFile
}

func createClientCert() string {
	certFilePath := path.Join(os.TempDir(), "client.crt")
	err := writeToFile([]byte("-----BEGIN CERTIFICATE-----\r\nMIIEHTCCAgegAwIBAgIRAKjHH3AIMByGKCTAiNiyLPYwCwYJKoZIhvcNAQELMBAx\r\nDjAMBgNVBAMTBWJic0NBMB4XDTE1MDkxNTIyMjkyM1oXDTE3MDkxNTIyMjkyNFow\r\nFTETMBEGA1UEAxMKYmJzIGNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC\r\nAQoCggEBAPKLCqMmuEKgTrHkFz5xAtUdaODN+tuYoqyx0fuXOBnPCC8UvC9+VAmf\r\nrur48XsSyz+1vPfyWNGVBficm/8adKgDxfZr8FF8pXO21q3McrCKq0K6hi3I71r2\r\nmeDTFFQ7FFW5ln9VmIbv2+7R2Dzc5W0eAjBziPH14Hi2StXiqTPKP4ygLppVib+p\r\nIwm0UHFZHCi2sb8mQgEDfQhvH7frTkP3auiII0k+ld1nFPsY3hXJrnOnPT01CO3/\r\nefb3ZUQQWZgkcYYj1RVCjT2s7c7DsjoAAflP3euVCK5+/YgFNFAXR2OUM4SjKK/0\r\nfy2Ef84J90vu7jeuPZXBNsO6GELF870CAwEAAaNxMG8wDgYDVR0PAQH/BAQDAgC4\r\nMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAdBgNVHQ4EFgQUwpu9qHz1\r\ndwc81frxHMzpmuEA7FowHwYDVR0jBBgwFoAU2HFnsP9JOWRvKvDs5YSkjx+r/LMw\r\nCwYJKoZIhvcNAQELA4ICAQAmAy3juekeu7jAQYI72+WMg73h5r9ebPswcj2RSAgl\r\nTOPCCec6yVW/SthxyD8NVfyoDOi6hn/ZvkHx5mSXx2VZ5Fr85C89qWNc551NYHyq\r\nzWTpy40gJBTqx3zFAAEz/IVHfVEBnJQ8CJjCplWqc9bar02ZH47ON4eHbtPnHIFZ\r\nQVXGfSZ81dvo0GKSvLjzPuRmYh1KYKFx32y7LT+6KwgzsaFdHBnlAic8CHq8E7u9\r\nnvsy2X6P7XM8K8mOVLy8JqZ6paGxmd0QmWz/5asTVFi3xDhwK/Kg72nS8FTmy4q4\r\nZGZenWttqYuXBANyP/GIt9YRD50tMHTWTrH3OGK1VNX4tQp7K72MFQuUX4YGywS+\r\nRpuywd5Zxqvxf2TKIGVhFdEWT0e2ecm8lJLbIPzs90b5OMUkNh0vEcpov5NxVGkc\r\nRPwM4znSztReHj6TDeM545ZD+IbtmsIvhnA0ZEwZKvTu+kS6OQt1eJ6dn7QrIExb\r\nB07oXUCMfVkYKtwuNwyZmRnu702tqdRrlLPKayTGl6j73ZeGh13BywhJAGgCjuXk\r\n6ZHORok5UN72pGTsqhGedlf32c8zzbE/Ize4rjyLFJmj/LeJWIyc4rSyG1KGYsFg\r\npkddZTkBCye6gM3HifJQjjXr19pF/RG3slnBDcWLJ0SX8kOfBG0SXcBFa22uGYOF\r\nIw==\r\n-----END CERTIFICATE-----"), certFilePath)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(fileExists(certFilePath)).To(BeTrue())
	return certFilePath
}

func createCACert() string {
	certFilePath := path.Join(os.TempDir(), "ca_cert.crt")
	err := writeToFile([]byte("-----BEGIN CERTIFICATE-----\r\nMIIE/TCCAuegAwIBAgIBATALBgkqhkiG9w0BAQswEDEOMAwGA1UEAxMFYmJzQ0Ew\r\nHhcNMTUwOTE1MjIyOTE4WhcNMjUwOTE1MjIyOTIxWjAQMQ4wDAYDVQQDEwViYnND\r\nQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAKjxJkuxVD5jrls2jXfB\r\nZsWd3HisgpdpgrTObOeJnrb6g4BB7GOSqMlZDEl0ROEBuT4Ax+tSEyhO8FgDR6Mq\r\nEy8h/HyOmCOsxt+0ZOlgmY04eGrSgkzhG41UiBEkezgFdxNCB8NZjTwwQmO2qjM7\r\nBsTS9SaEh11HdpIhoeu22aqXuP0r56ZaRC7rfPb+U9SaWaygwMfgXZ7ZDBizHz+n\r\ngRSvQ+KnvHG1nZGR+vwuNikBdby8YRBVXaGjF1I7uZh/kcPm2XX9RwHaXSIgGyuK\r\nC+YJy95L4WdX2sgm8Mm+mhIKRnGggBbmUmbDT8URkYIu11YEI/FqH/+WmEPv0UC7\r\nU1rSVkQVhlHgO6Ohjoe251jw9U1UR0qXsfI/2maPESxJW2FDXOrBCzMK0/Us+y7M\r\nrBRLhLkYJmv9GUFQG1M3eOfP6VIMMm6wZ1+2untcI7Eb+HZxhO91ddYlKNbFpZ7P\r\nf0P0GuopPE6kzX3gFoivEHxIslumeoVDgMzQ4uj1TYGmOtjuiD48kIrVaeEKUcxN\r\n7YzSt3tTZ+a1GKqFcuj+g/rbUYLBT5Ztj89O3AahnCzCymOJ3EkWQ4aJzdAs3KEG\r\nRxGs2zzsBKkTp+UXXv4q/GrZ+J/PjqY9285TaQx3MZmdLdIyNoh6UwwFPdyEYsTv\r\nxhtJb5NdjY9K56mkeVEkfuGtAgMBAAGjZjBkMA4GA1UdDwEB/wQEAwIABjASBgNV\r\nHRMBAf8ECDAGAQH/AgEAMB0GA1UdDgQWBBTYcWew/0k5ZG8q8OzlhKSPH6v8szAf\r\nBgNVHSMEGDAWgBTYcWew/0k5ZG8q8OzlhKSPH6v8szALBgkqhkiG9w0BAQsDggIB\r\nAFt3ueVxYhu5vT1IKL/xIuxfl8SXZqaJSg35DqJ6FlEDU+E/mjflrPMsV5Iz5ycd\r\nJMO3hN9ipilkfx5m7gTIDcxl0izej2jlI2uncjLT6MsPI1+LsRxyVDR4+MDvM7ce\r\nmyfpIPNQlGQI/cTkmOT+tTaffwf6PLcvT/HvJivax/y0tIsCIqtTSoM6eoi6D9jN\r\nn/VkMsZpaxxIt0nm87ZgcWA6IVPdtO51eLWlJyfz8/V8f/ySARUMdMSVkFiS6OMS\r\nnxsrQGPLOOWTYepV6XD4GP9zDYL4aLArGfWprq79KHAtRYtGHixgcxFgbfBnon2y\r\n6HG1vDa/sVFrleSwBRsCtVRgYvAShdn50hL4JgSn8OjkkTVB1wz74bqCj001RHfS\r\ndxKhfzBPQsqsdGCMZKkRGUpUavM3qW/UAxbYgkjcS04hzmjyC/I1sKpDebQJyX9i\r\n66F3zR7eRzwH7Y8s5PTo+dYZJmNxtN7vJKq++8Cg707XUzBT/U2SQV84TOsZO70Q\r\nHl7GKY3NdpVEslyiwMdi6DyhTH+MV3HMkEds16wCRNAVriSXPeg/GYNhQqcdTceU\r\nI0YSumzEeQMcFbg0LUYayZ9PlhPgLosMba9BDK/K244OZvmGyRr1ANnnASsQg4cK\r\nvsHDEV4jBWxHAw41ArfNLg9vA8ojf/1EU4E2d5GU5fVe\r\n-----END CERTIFICATE-----"), certFilePath)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(fileExists(certFilePath)).To(BeTrue())
	return certFilePath
}

func createClientKey() string {
	certFilePath := path.Join(os.TempDir(), "client.key")
	err := writeToFile([]byte("-----BEGIN RSA PRIVATE KEY-----\r\nMIIEpQIBAAKCAQEA8osKoya4QqBOseQXPnEC1R1o4M3625iirLHR+5c4Gc8ILxS8\r\nL35UCZ+u6vjxexLLP7W89/JY0ZUF+Jyb/xp0qAPF9mvwUXylc7bWrcxysIqrQrqG\r\nLcjvWvaZ4NMUVDsUVbmWf1WYhu/b7tHYPNzlbR4CMHOI8fXgeLZK1eKpM8o/jKAu\r\nmlWJv6kjCbRQcVkcKLaxvyZCAQN9CG8ft+tOQ/dq6IgjST6V3WcU+xjeFcmuc6c9\r\nPTUI7f959vdlRBBZmCRxhiPVFUKNPaztzsOyOgAB+U/d65UIrn79iAU0UBdHY5Qz\r\nhKMor/R/LYR/zgn3S+7uN649lcE2w7oYQsXzvQIDAQABAoIBAEvVk3bdpWEXlGNk\r\niKv6U8NklaUsYhIFEF/knV4Hsv/Gzq1B03EaE5aKufs36PDtOGVsInB38rNc3+gS\r\nt2e00uKxg1T//LzNt0GN2mOu9/Eg+lk7zrZEDCqpzgUQmluXuUzwYRDhJ3aRSnfK\r\nXszw2D8c0dxqU1gr44p6nL1xSCwrpYhj1z2JN8IN0wQmfOE2kphZnnrn8TpJMTjy\r\n00IW9LdMyGUXiCQbN29eDY108LhXWzcs5jXdb/APBpI4HkmuZVEJiuV6iM6qeVX9\r\nFbVgpQivBAFpGiILeboFrUo+qOpto2EEPu0KoyoqMGCPAoQwri0xa/9j3OE6iTDk\r\n4WbbJ+ECgYEA/abBmPz0L+T9H/cZvufsjhW9WthCzAuereTzXdEhDk5kr5PJxbMp\r\nDIu9c4vzxJDwpaEEOkBxPoptG/QG5AZ3SCsevQr5kqfZo+OMIWmheb2uL9z8k5hx\r\nwq7vvLQ0gLO9L1oZVmm2/F1J8Nku9M5PkWHmBjbHN1pl5HqOlsxODEkCgYEA9Mn0\r\nbWTnpnJ4ee8kO0/WOcIDyTV9s4XMaN3lLFhtI8ig6lt6eWA4mSpqRADCs0hngECD\r\niUfal+KHbYAGsvdm7roB+GgoS2Hv/SxSjOxhsaQ1xvYYM+jzIIy3WlCPtaIxHkm5\r\noXVnh4BcvbHUP7yi/lvg8BbK3HDPcPn5Xc9p49UCgYEA5T+x+fOlPyRXImzSeBhl\r\nVIWRfmm29XQLFl+3FTPODIANwCJyWpxynUQvFh+HUkEtPoUorP1RXJT/yCPllnHB\r\nnRhbz7/7kPDjY5xlKk2uA7nLlLbGER/WsX4qbwLv8OKCOinUfKVPHQezrFqedeOB\r\nRoSUwUkBBKZPMRETjndYkwECgYEAkzwh2+a0euYhVt4jQdWcefMbidu1ttREhdLp\r\ntEmfo8VaHHxXZ0gb4uyjLDH06hcjwf2L4HeqoG6tnIxD+0NZ0z9oTfyAOA85ZWNS\r\nZ9cKT+oAOqLtHdQA4NQiuJz6Q3rB5oDbuaS/V746ihK7IncY5rtmyaI79GmaLE7+\r\n0ZEfFN0CgYEA8HTclSd7Z58J2F0qp4xZuD6nK/894LiIpAQAljfVVplrFSK3tyQD\r\nXCAuu6OuNcVb7RktBAzVJksp+fT5BeyYYm3KviaD2eXmRnJtpUSzN12VieyBX7Fx\r\nBSgiGpK7DUMYgC6mggfL8bPrhWG3B39Q+VC8B6K/Y29xe8HZQThhick=\r\n-----END RSA PRIVATE KEY-----"), certFilePath)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(fileExists(certFilePath)).To(BeTrue())
	return certFilePath
}

func writeToFile(data []byte, fileName string) error {
	var file *os.File
	var err error
	file, err = os.Create(fileName)
	if err != nil {
		return err
	}

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}

func fileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	if err == nil {
		return true
	}
	var result bool = os.IsExist(err)
	return result
}
