package locket

import (
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ClientLocketConfig struct {
	LocketAddress        string `json:"locket_address,omitempty" yaml:"locket_address,omitempty"`
	LocketCACertFile     string `json:"locket_ca_cert_file,omitempty" yaml:"locket_ca_cert_file,omitempty"`
	LocketClientCertFile string `json:"locket_client_cert_file,omitempty" yaml:"locket_client_cert_file,omitempty"`
	LocketClientKeyFile  string `json:"locket_client_key_file,omitempty" yaml:"locket_client_key_file,omitempty"`
}

func NewClient(logger lager.Logger, config ClientLocketConfig) (models.LocketClient, error) {
	locketTLSConfig, err := cfhttp.NewTLSConfig(config.LocketClientCertFile, config.LocketClientKeyFile, config.LocketCACertFile)
	if err != nil {
		logger.Error("failed-to-open-tls-config", err, lager.Data{"keypath": config.LocketClientKeyFile, "certpath": config.LocketClientCertFile, "capath": config.LocketCACertFile})
		return nil, err
	}

	conn, err := grpc.Dial(config.LocketAddress, grpc.WithTransportCredentials(credentials.NewTLS(locketTLSConfig)))
	if err != nil {
		return nil, err
	}
	return models.NewLocketClient(conn), nil
}
