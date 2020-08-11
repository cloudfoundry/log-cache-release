package main

import (
	"time"

	"code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/log-cache/internal/config"
)

type CAPI struct {
	Addr       string `env:"CAPI_ADDR,        required, report"`
	CAPath     string `env:"CAPI_CA_PATH,               report"`
	CommonName string `env:"CAPI_COMMON_NAME,           report"`
}

type UAA struct {
	ClientID     string `env:"UAA_CLIENT_ID,"`
	ClientSecret string `env:"UAA_CLIENT_SECRET,"`
	Addr         string `env:"UAA_ADDR,          required, report"`
	CAPath       string `env:"UAA_CA_PATH,                 report"`
}

type Config struct {
	LogCacheGatewayAddr     string        `env:"LOG_CACHE_GATEWAY_ADDR, required, report"`
	Addr                    string        `env:"ADDR,                   required, report"`
	InternalIP              string        `env:"INTERNAL_IP,                      report"`
	CertPath                string        `env:"EXTERNAL_CERT,                    report"`
	KeyPath                 string        `env:"EXTERNAL_KEY,                     report"`
	SkipCertVerify          bool          `env:"SKIP_CERT_VERIFY,                 report"`
	ProxyCAPath             string        `env:"PROXY_CA_PATH,					  report"`
	SecurityEventLog        string        `env:"SECURITY_EVENT_LOG,               report"`
	TokenPruningInterval    time.Duration `env:"TOKEN_PRUNING_INTERVAL,           report"`
	CacheExpirationInterval time.Duration `env:"CACHE_EXPIRATION_INTERVAL,        report"`

	CAPI          CAPI
	UAA           UAA
	MetricsServer config.MetricsServer
}

func LoadConfig() (*Config, error) {
	cfg := Config{
		SkipCertVerify:          false,
		Addr:                    ":8083",
		InternalIP:              "0.0.0.0",
		LogCacheGatewayAddr:     "localhost:8081",
		CacheExpirationInterval: time.Minute,
		MetricsServer: config.MetricsServer{
			Port: 6065,
		},
	}

	err := envstruct.Load(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
