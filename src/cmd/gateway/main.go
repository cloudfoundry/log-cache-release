package main

import (
	"code.cloudfoundry.org/go-loggregator/metrics"
	"log"
	"os"

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

	metrics.NewRegistry(
		log.New(os.Stderr, "[METRICS] ", log.LstdFlags),
		metrics.WithTLSServer(
			int(cfg.MetricsServer.Port),
			cfg.MetricsServer.CertFile,
			cfg.MetricsServer.KeyFile,
			cfg.MetricsServer.CAFile,
		),
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
