package main

import (
	"log"
	"os"

	"code.cloudfoundry.org/go-loggregator/metrics"

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

	metricServerOption := metrics.WithServer(int(cfg.MetricsServer.Port))
	if cfg.MetricsServer.CAFile != "" {
		metricServerOption = metrics.WithTLSServer(
			int(cfg.MetricsServer.Port),
			cfg.MetricsServer.CertFile,
			cfg.MetricsServer.KeyFile,
			cfg.MetricsServer.CAFile,
		)
	}
	metrics.NewRegistry(
		log.New(os.Stderr, "[METRICS] ", log.LstdFlags),
		metricServerOption,
	)

	gateway := NewGateway(cfg.LogCacheAddr, cfg.Addr, cfg.ProxyCertPath, cfg.ProxyKeyPath,
		WithGatewayLogger(log.New(os.Stderr, "[GATEWAY] ", log.LstdFlags)),
		WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(cfg.TLS.Credentials("log-cache")),
		),
		WithGatewayVersion(cfg.Version),
		WithGatewayBlock(),
	)

	gateway.Start()
}
