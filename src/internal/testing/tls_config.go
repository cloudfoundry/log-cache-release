package testing

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
)

func NewTLSConfig(caPath, certPath, keyPath, cn string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         cn,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: false,
	}

	caCertBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, errors.New("cannot parse ca cert")
	}

	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}
