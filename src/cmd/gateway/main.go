package main

import (
	"log"
	"os"

	metrics "code.cloudfoundry.org/go-metric-registry"

	_ "net/http/pprof"

	. "code.cloudfoundry.org/log-cache/internal/gateway"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting Log Cache Gateway...")
	defer log.Print("Closing Log Cache Gateway.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
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
	metrics.NewRegistry(
		log.New(os.Stderr, "[METRICS] ", log.LstdFlags),
		metricServerOption,
	)

	gatewayOptions := []GatewayOption{
		WithGatewayLogger(log.New(os.Stderr, "[GATEWAY] ", log.LstdFlags)),
		WithGatewayVersion(cfg.Version),
		WithGatewayBlock(),
	}

	if cfg.ProxyCertPath != "" || cfg.ProxyKeyPath != "" {
		gatewayOptions = append(gatewayOptions, WithGatewayTLSServer(cfg.ProxyCertPath, cfg.ProxyKeyPath))
	}
	if cfg.TLS.HasAnyCredential() {
		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(cfg.TLS.Credentials("log-cache")),
		),
		)
	} else {
		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithInsecure(),
		),
		)

	}

	gateway := NewGateway(cfg.LogCacheAddr, cfg.Addr, gatewayOptions...)

	gateway.Start()
}
