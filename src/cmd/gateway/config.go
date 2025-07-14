package main

import (
	"log/slog"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/log-cache/internal/config"
	"code.cloudfoundry.org/log-cache/internal/tls"
)

var buildVersion string

// Config is the configuration for a LogCache Gateway.
type Config struct {
	Addr          string `env:"ADDR,           required, report"`
	LogCacheAddr  string `env:"LOG_CACHE_ADDR, required, report"`
	ProxyCertPath string `env:"PROXY_CERT_PATH,          report"`
	ProxyKeyPath  string `env:"PROXY_KEY_PATH,           report"`
	Version       string `env:"-,                        report"`

	TLS           tls.TLS
	MetricsServer config.MetricsServer
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
		slog.Error("Failed to write report", "error", err)
	}

	return &c, nil
}
