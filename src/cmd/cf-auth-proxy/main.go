package main

import (
	"log"
	"net"
	"net/http"

	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	metrics "code.cloudfoundry.org/go-metric-registry"

	"crypto/x509"

	"code.cloudfoundry.org/go-envstruct"
	client "code.cloudfoundry.org/go-log-cache/v2"
	"code.cloudfoundry.org/log-cache/internal/auth"
	. "code.cloudfoundry.org/log-cache/internal/cfauthproxy"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"code.cloudfoundry.org/log-cache/internal/pprof"
	"code.cloudfoundry.org/log-cache/internal/promql"
)

func main() {
	var loggr *log.Logger

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %s", err)
	}

	if cfg.UseRFC339 {
		loggr = log.New(new(plumbing.LogWriter), "", 0)
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		loggr = log.New(os.Stderr, "", log.LstdFlags)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	loggr.Print("Starting Log Cache CF Auth Reverse Proxy...")
	defer loggr.Print("Closing Log Cache CF Auth Reverse Proxy.")

	err = envstruct.WriteReport(cfg)
	if err != nil {
		log.Printf("Failed to print a report of the from environment: %s\n", err)
	}
	metricServerOption := metrics.WithTLSServer(
		int(cfg.MetricsServer.Port),
		cfg.MetricsServer.CertFile,
		cfg.MetricsServer.KeyFile,
		cfg.MetricsServer.CAFile,
	)
	if cfg.MetricsServer.CAFile == "" {
		metricServerOption = metrics.WithPublicServer(int(cfg.MetricsServer.Port))
	}

	metrics := metrics.NewRegistry(
		loggr,
		metricServerOption,
	)
	if cfg.MetricsServer.DebugMetrics {
		metrics.RegisterDebugMetrics()
		go func() { loggr.Println("PPROF SERVER STOPPED " + pprof.RunServer(cfg.MetricsServer.PprofPort).Error()) }()
	}

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

	gatewayURL, err := url.Parse(cfg.LogCacheGatewayAddr)
	if err != nil {
		loggr.Fatalf("failed to parse gateway address: %s", err)
	}

	capiClient := auth.NewCAPIClient(
		cfg.CAPI.Addr,
		buildCAPIClient(cfg, loggr),
		metrics,
		loggr,
		auth.WithCacheExpirationInterval(cfg.CacheExpirationInterval),
	)

	// Calls to /api/v1/meta get sent to the gateway, but not through the
	// reverse proxy like everything else. As a result, we also need to set
	// the Transport here to ensure the correct root CA is available.
	metaHTTPClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	if cfg.ProxyCAPath != "" {
		proxyCACertPool := loadCA(cfg.ProxyCAPath, loggr)
		metaHTTPClient.Transport = NewTransportWithRootCA(proxyCACertPool)
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
	}

	if cfg.ProxyCAPath != "" {
		proxyCACertPool := loadCA(cfg.ProxyCAPath, loggr)
		proxyOptions = append(proxyOptions, WithCFAuthProxyCACertPool(proxyCACertPool))
	}

	if cfg.PromQLUnimplemented {
		proxyOptions = append(proxyOptions, WithPromMiddleware(promql.UnimplementedMiddleware))
	}

	if cfg.CertPath == "" && cfg.KeyPath == "" {
		proxyOptions = append(proxyOptions, WithCFAuthProxyTLSDisabled())
	} else {
		proxyOptions = append(proxyOptions, WithCFAuthProxyTLSServer(cfg.CertPath, cfg.KeyPath))
	}

	proxy := NewCFAuthProxy(
		gatewayURL.String(),
		cfg.Addr,
		proxyOptions...,
	)

	if cfg.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(cfg.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			loggr.Panicf("Unable to open access log: %s", err)
		}

		//nolint:errcheck
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

	proxy.Start(uaaClient.RefreshTokenKeys)
}

func buildUAAClient(cfg *Config, loggr *log.Logger) *http.Client {
	uaaClient := &http.Client{
		Timeout: 20 * time.Second,
	}

	if cfg.UAA.CAPath == "" {
		return uaaClient
	}

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(cfg.UAA.CAPath),
	)
	if err != nil {
		panic(err)
	}
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify

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

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Client(
		tlsconfig.WithAuthorityFromFile(cfg.CAPI.CAPath),
		tlsconfig.WithServerName(cfg.CAPI.CommonName),
	)
	if err != nil {
		panic(err)
	}
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify

	capiClient.Transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return capiClient
}

func loadCA(caCertPath string, loggr *log.Logger) *x509.CertPool {
	caCert, err := os.ReadFile(caCertPath)
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
