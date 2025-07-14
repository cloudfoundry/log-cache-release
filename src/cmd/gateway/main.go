package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/tlsconfig"

	"net/http"

	//nolint: gosec
	_ "net/http/pprof"

	. "code.cloudfoundry.org/log-cache/internal/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func main() {
	slog.Info("Starting Log Cache Gateway...")
	defer slog.Info("Log Cache Gateway stopped.")

	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
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
	m := metrics.NewRegistry(
		slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo),
		metricServerOption,
	)
	if cfg.MetricsServer.DebugMetrics {
		m.RegisterDebugMetrics()
		pprofServer := &http.Server{
			Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.MetricsServer.PprofPort),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 2 * time.Second,
		}
		go func() { slog.Info("pprof server stopped", "error", pprofServer.ListenAndServe().Error()) }()
	}

	gatewayOptions := []GatewayOption{
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
