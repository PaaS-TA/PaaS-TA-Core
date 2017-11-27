package containerstore

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/garden"
	loggregator_v2 "code.cloudfoundry.org/go-loggregator/compatibility"
	"code.cloudfoundry.org/lager"
)

const (
	CredCreationSucceededCount    = "CredCreationSucceededCount"
	CredCreationSucceededDuration = "CredCreationSucceededDuration"
	CredCreationFailedCount       = "CredCreationFailedCount"
)

//go:generate counterfeiter -o containerstorefakes/fake_cred_manager.go . CredManager
type CredManager interface {
	CreateCredDir(lager.Logger, executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error)
	Runner(lager.Logger, executor.Container) ifrit.Runner
}

type noopManager struct{}

func NewNoopCredManager() CredManager {
	return &noopManager{}
}

func (c *noopManager) CreateCredDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	return nil, nil, nil
}

func (c *noopManager) Runner(lager.Logger, executor.Container) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		<-signals
		return nil
	})
}

type credManager struct {
	logger             lager.Logger
	metronClient       loggregator_v2.IngressClient
	credDir            string
	validityPeriod     time.Duration
	entropyReader      io.Reader
	clock              clock.Clock
	CaCert             *x509.Certificate
	privateKey         *rsa.PrivateKey
	containerMountPath string
}

func NewCredManager(
	logger lager.Logger,
	metronClient loggregator_v2.IngressClient,
	credDir string,
	validityPeriod time.Duration,
	entropyReader io.Reader,
	clock clock.Clock,
	CaCert *x509.Certificate,
	privateKey *rsa.PrivateKey,
	containerMountPath string,
) CredManager {
	return &credManager{
		logger:             logger,
		metronClient:       metronClient,
		credDir:            credDir,
		validityPeriod:     validityPeriod,
		entropyReader:      entropyReader,
		clock:              clock,
		CaCert:             CaCert,
		privateKey:         privateKey,
		containerMountPath: containerMountPath,
	}
}

func calculateCredentialRotationPeriod(validityPeriod time.Duration) time.Duration {
	if validityPeriod > 4*time.Hour {
		return validityPeriod - 30*time.Minute
	} else {
		eighth := validityPeriod / 8
		return validityPeriod - eighth
	}
}

func (c *credManager) Runner(logger lager.Logger, container executor.Container) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		logger = logger.Session("cred-manager-runner")
		logger.Info("starting")
		defer logger.Info("finished")

		start := c.clock.Now()
		err := c.generateCreds(logger, container)
		duration := c.clock.Since(start)
		if err != nil {
			logger.Error("failed-to-generate-credentials", err)
			c.metronClient.IncrementCounter(CredCreationFailedCount)
			return err
		}

		c.metronClient.IncrementCounter(CredCreationSucceededCount)
		c.metronClient.SendDuration(CredCreationSucceededDuration, duration)

		rotationDuration := calculateCredentialRotationPeriod(c.validityPeriod)
		regenCertTimer := c.clock.NewTimer(rotationDuration)

		close(ready)
		for {
			select {
			case <-regenCertTimer.C():
				logger = logger.Session("regenerating-cert-and-key")
				logger.Debug("started")
				start := c.clock.Now()
				err = c.generateCreds(logger, container)
				duration := c.clock.Since(start)
				if err != nil {
					logger.Error("failed-to-generate-credentials", err)
					c.metronClient.IncrementCounter(CredCreationFailedCount)
					return err
				}
				c.metronClient.IncrementCounter(CredCreationSucceededCount)
				c.metronClient.SendDuration(CredCreationSucceededDuration, duration)

				rotationDuration = calculateCredentialRotationPeriod(c.validityPeriod)
				regenCertTimer.Reset(rotationDuration)
				logger.Debug("completed")
			case <-signals:
				logger.Info("signaled")
				return c.removeCreds(logger, container)
			}
		}
	})
}

func (c *credManager) CreateCredDir(logger lager.Logger, container executor.Container) ([]garden.BindMount, []executor.EnvironmentVariable, error) {
	logger = logger.Session("create-cred-dir")
	logger.Info("starting")
	defer logger.Info("complete")

	containerDir := filepath.Join(c.credDir, container.Guid)
	err := os.Mkdir(containerDir, 0755)
	if err != nil {
		return nil, nil, err
	}

	return []garden.BindMount{
			{
				SrcPath: containerDir,
				DstPath: c.containerMountPath,
				Mode:    garden.BindMountModeRO,
				Origin:  garden.BindMountOriginHost,
			},
		}, []executor.EnvironmentVariable{
			{Name: "CF_INSTANCE_CERT", Value: path.Join(c.containerMountPath, "instance.crt")},
			{Name: "CF_INSTANCE_KEY", Value: path.Join(c.containerMountPath, "instance.key")},
		}, nil
}

const (
	certificatePEMBlockType = "CERTIFICATE"
	privateKeyPEMBlockType  = "RSA PRIVATE KEY"
)

func (c *credManager) generateCreds(logger lager.Logger, container executor.Container) error {
	logger = logger.Session("generating-credentials")
	logger.Info("starting")
	defer logger.Info("complete")

	logger.Debug("generating-private-key")
	privateKey, err := rsa.GenerateKey(c.entropyReader, 2048)
	if err != nil {
		return err
	}
	logger.Debug("generated-private-key")

	now := c.clock.Now()
	template := createCertificateTemplate(container.InternalIP,
		container.Guid,
		now,
		now.Add(c.validityPeriod),
		container.CertificateProperties.OrganizationalUnit,
	)

	logger.Debug("generating-serial-number")
	guid, err := uuid.NewV4()
	if err != nil {
		logger.Error("failed-to-generate-uuid", err)
		return err
	}
	logger.Debug("generated-serial-number")

	guidBytes := [16]byte(*guid)
	template.SerialNumber.SetBytes(guidBytes[:])

	logger.Debug("generating-certificate")
	certBytes, err := x509.CreateCertificate(c.entropyReader, template, c.CaCert, privateKey.Public(), c.privateKey)
	if err != nil {
		return err
	}
	logger.Debug("generated-certificate")

	instanceKeyPath := filepath.Join(c.credDir, container.Guid, "instance.key")
	tmpInstanceKeyPath := instanceKeyPath + ".tmp"
	certificatePath := filepath.Join(c.credDir, container.Guid, "instance.crt")
	tmpCertificatePath := certificatePath + ".tmp"

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	instanceKey, err := os.Create(tmpInstanceKeyPath)
	if err != nil {
		return err
	}

	defer instanceKey.Close()

	err = pemEncode(privateKeyBytes, privateKeyPEMBlockType, instanceKey)
	if err != nil {
		return err
	}

	certificate, err := os.Create(tmpCertificatePath)
	if err != nil {
		return err
	}

	defer certificate.Close()
	err = pemEncode(certBytes, certificatePEMBlockType, certificate)
	if err != nil {
		return err
	}

	err = pemEncode(c.CaCert.Raw, certificatePEMBlockType, certificate)
	if err != nil {
		return err
	}

	err = instanceKey.Close()
	if err != nil {
		return err
	}

	err = certificate.Close()
	if err != nil {
		return err
	}

	err = os.Rename(tmpInstanceKeyPath, instanceKeyPath)
	if err != nil {
		return err
	}

	return os.Rename(tmpCertificatePath, certificatePath)
}

func (c *credManager) removeCreds(logger lager.Logger, container executor.Container) error {
	logger = logger.Session("remove-credentials")
	logger.Info("starting")
	defer logger.Info("complete")

	err := os.RemoveAll(filepath.Join(c.credDir, container.Guid))
	if err != nil {
		return err
	}

	return nil
}

func pemEncode(bytes []byte, blockType string, writer io.Writer) error {
	block := &pem.Block{
		Type:  blockType,
		Bytes: bytes,
	}
	return pem.Encode(writer, block)
}

func createCertificateTemplate(ipaddress, guid string, notBefore, notAfter time.Time, organizationalUnits []string) *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:         guid,
			OrganizationalUnit: organizationalUnits,
		},
		IPAddresses: []net.IP{net.ParseIP(ipaddress)},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageKeyAgreement,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
}

func certFromFile(certFile string) (*x509.Certificate, error) {
	data, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	var block *pem.Block
	block, _ = pem.Decode(data)
	certs, err := x509.ParseCertificates(block.Bytes)
	if err != nil {
		return nil, err
	}
	cert := certs[0]
	return cert, nil
}
