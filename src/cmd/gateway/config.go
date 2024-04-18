package main

import (
	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/log-cache/internal/config"
)

var buildVersion string

// Config is the configuration for a LogCache Gateway.
type Config struct {
	Addr          string `env:"ADDR,           required, report"`
	LogCacheAddr  string `env:"LOG_CACHE_ADDR, required, report"`
	ProxyCertPath string `env:"PROXY_CERT_PATH,          report"`
	ProxyKeyPath  string `env:"PROXY_KEY_PATH,           report"`
	Version       string `env:"-,                        report"`

	TLS           config.TLS
	MetricsServer config.MetricsServer
	UseRFC339     bool `env:"USE_RFC339"`
}

// LoadConfig creates Config object from environment variables
func LoadConfig() (*Config, error) {
	c := Config{
		Addr:         ":8081",
		LogCacheAddr: "localhost:8080",
		MetricsServer: config.MetricsServer{
			Port: 6063,
		},
	}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	c.Version = buildVersion

	err := envstruct.WriteReport(&c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
