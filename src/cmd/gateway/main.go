package main

import (
	"fmt"
	"log"
	"os"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/tlsconfig"

	"net/http"

	//nolint: gosec
	_ "net/http/pprof"

	. "code.cloudfoundry.org/log-cache/internal/gateway"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var metricsLoggr *log.Logger
	var gatewayLoggr *log.Logger

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	if cfg.UseRFC339 {
		metricsLoggr = log.New(new(plumbing.LogWriter), "[METRICS] ", 0)
		gatewayLoggr = log.New(new(plumbing.LogWriter), "[GATEWAY] ", 0)
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		metricsLoggr = log.New(os.Stderr, "[METRICS] ", log.LstdFlags)
		gatewayLoggr = log.New(os.Stderr, "[GATEWAY] ", log.LstdFlags)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	log.Print("Starting Log Cache Gateway...")
	defer log.Print("Closing Log Cache Gateway.")

	metricServerOption := metrics.WithTLSServer(
		int(cfg.MetricsServer.Port),
		cfg.MetricsServer.CertFile,
		cfg.MetricsServer.KeyFile,
		cfg.MetricsServer.CAFile,
	)
	if cfg.MetricsServer.CAFile == "" {
		metricServerOption = metrics.WithPublicServer(int(cfg.MetricsServer.Port))
	}
	m := metrics.NewRegistry(
		metricsLoggr,
		metricServerOption,
	)
	if cfg.MetricsServer.DebugMetrics {
		m.RegisterDebugMetrics()
		pprofServer := &http.Server{
			Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.MetricsServer.PprofPort),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 2 * time.Second,
		}
		go func() { log.Println("PPROF SERVER STOPPED " + pprofServer.ListenAndServe().Error()) }()
	}

	gatewayOptions := []GatewayOption{
		WithGatewayLogger(gatewayLoggr),
		WithGatewayVersion(cfg.Version),
		WithGatewayBlock(),
	}

	if cfg.ProxyCertPath != "" || cfg.ProxyKeyPath != "" {
		gatewayOptions = append(gatewayOptions, WithGatewayTLSServer(cfg.ProxyCertPath, cfg.ProxyKeyPath))
	}
	if cfg.TLS.HasAnyCredential() {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(cfg.TLS.CertPath, cfg.TLS.KeyPath),
		).Client(
			tlsconfig.WithAuthorityFromFile(cfg.TLS.CAPath),
			tlsconfig.WithServerName("log-cache"),
		)
		if err != nil {
			panic(err)
		}

		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
		),
		)
	} else {
		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
		),
		)

	}

	gateway := NewGateway(cfg.LogCacheAddr, cfg.Addr, gatewayOptions...)

	gateway.Start()
}
