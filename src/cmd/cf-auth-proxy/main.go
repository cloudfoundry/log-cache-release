package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/go-loggregator/metrics"

	"crypto/x509"

	"code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/log-cache/internal/auth"
	. "code.cloudfoundry.org/log-cache/internal/cfauthproxy"
	"code.cloudfoundry.org/log-cache/internal/promql"
	sharedtls "code.cloudfoundry.org/log-cache/internal/tls"
	"code.cloudfoundry.org/log-cache/pkg/client"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	loggr := log.New(os.Stderr, "", log.LstdFlags)
	loggr.Print("Starting Log Cache CF Auth Reverse Proxy...")
	defer loggr.Print("Closing Log Cache CF Auth Reverse Proxy.")

	cfg, err := LoadConfig()
	if err != nil {
		loggr.Fatalf("failed to load config: %s", err)
	}
	envstruct.WriteReport(cfg)

	metricServerOption := metrics.WithServer(int(cfg.MetricsServer.Port))

	if cfg.MetricsServer.CAFile != "" {
		metricServerOption = metrics.WithTLSServer(
			int(cfg.MetricsServer.Port),
			cfg.MetricsServer.CertFile,
			cfg.MetricsServer.KeyFile,
			cfg.MetricsServer.CAFile,
		)
	}

	metrics := metrics.NewRegistry(
		loggr,
		metricServerOption,
	)

	var options []auth.UAAOption
	if cfg.UAA.ClientID != "" && cfg.UAA.ClientSecret != "" {
		options = append(options, auth.WithBasicAuth(cfg.UAA.ClientID, cfg.UAA.ClientSecret))
	}
	uaaClient := auth.NewUAAClient(
		cfg.UAA.Addr,
		buildUAAClient(cfg, loggr),
		metrics,
		loggr,
		options...,
	)

	// try to get our first token key, but bail out if we can't talk to UAA
	err = uaaClient.RefreshTokenKeys()
	if err != nil {
		loggr.Fatalf("failed to fetch token from UAA: %s", err)
	}

	gatewayURL, err := url.Parse(cfg.LogCacheGatewayAddr)
	if err != nil {
		loggr.Fatalf("failed to parse gateway address: %s", err)
	}

	// Force communication with the gateway to happen via HTTPS, regardless of
	// the scheme provided in the config
	gatewayURL.Scheme = "https"

	capiClient := auth.NewCAPIClient(
		cfg.CAPI.Addr,
		buildCAPIClient(cfg, loggr),
		metrics,
		loggr,
		auth.WithCacheExpirationInterval(cfg.CacheExpirationInterval),
	)

	proxyCACertPool := loadCA(cfg.ProxyCAPath, loggr)

	// Calls to /api/v1/meta get sent to the gateway, but not through the
	// reverse proxy like everything else. As a result, we also need to set
	// the Transport here to ensure the correct root CA is available.
	metaHTTPClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: NewTransportWithRootCA(proxyCACertPool),
	}

	metaFetcher := client.NewClient(
		gatewayURL.String(),
		client.WithHTTPClient(metaHTTPClient),
	)

	middlewareProvider := auth.NewCFAuthMiddlewareProvider(
		uaaClient,
		capiClient,
		metaFetcher,
		promql.ExtractSourceIds,
		capiClient,
	)

	proxyOptions := []CFAuthProxyOption{
		WithAuthMiddleware(middlewareProvider.Middleware),
		WithCFAuthProxyBlock(),
	}

	if cfg.DisableTLSServer {
		proxyOptions = append(proxyOptions, WithCFAuthProxyTLSDisabled())
	}

	proxy := NewCFAuthProxy(
		gatewayURL.String(),
		cfg.Addr,
		cfg.CertPath,
		cfg.KeyPath,
		proxyCACertPool,
		proxyOptions...,
	)

	if cfg.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(cfg.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			loggr.Panicf("Unable to open access log: %s", err)
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()

		_, localPort, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			loggr.Panicf("Unable to determine local port: %s", err)
		}

		accessLogger := auth.NewAccessLogger(accessLog)
		accessMiddleware := auth.NewAccessMiddleware(accessLogger, cfg.InternalIP, localPort)
		WithAccessMiddleware(accessMiddleware)(proxy)
	}

	proxy.Start()
}

func buildUAAClient(cfg *Config, loggr *log.Logger) *http.Client {
	uaaClient := &http.Client{
		Timeout: 20 * time.Second,
	}

	if cfg.UAA.CAPath == "" {
		return uaaClient
	}
	tlsConfig := sharedtls.NewBaseTLSConfig()
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify

	tlsConfig.RootCAs = loadCA(cfg.UAA.CAPath, loggr)

	uaaClient.Transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return uaaClient
}

func buildCAPIClient(cfg *Config, loggr *log.Logger) *http.Client {
	capiClient := &http.Client{
		Timeout: 20 * time.Second,
	}

	if cfg.CAPI.CAPath == "" {
		return capiClient
	}

	tlsConfig := sharedtls.NewBaseTLSConfig()
	tlsConfig.ServerName = cfg.CAPI.CommonName

	tlsConfig.RootCAs = loadCA(cfg.CAPI.CAPath, loggr)

	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify
	capiClient.Transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return capiClient
}

func loadCA(caCertPath string, loggr *log.Logger) *x509.CertPool {
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		loggr.Fatalf("failed to read CA certificate: %s", err)
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		loggr.Fatal("failed to parse CA certificate.")
	}

	return certPool
}
