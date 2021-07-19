package main

import (
	"log"
	"os"

	metrics "code.cloudfoundry.org/go-metric-registry"

	_ "net/http/pprof"

	. "code.cloudfoundry.org/log-cache/internal/gateway"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"google.golang.org/grpc"
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
	metrics.NewRegistry(
		metricsLoggr,
		metricServerOption,
	)

	gatewayOptions := []GatewayOption{
		WithGatewayLogger(gatewayLoggr),
		WithGatewayVersion(cfg.Version),
		WithGatewayBlock(),
	}

	if cfg.ProxyCertPath != "" || cfg.ProxyKeyPath != "" {
		gatewayOptions = append(gatewayOptions, WithGatewayTLSServer(cfg.ProxyCertPath, cfg.ProxyKeyPath))
	}
	if cfg.TLS.HasAnyCredential() {
		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(cfg.TLS.Credentials("log-cache")),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*8)),
		),
		)
	} else {
		gatewayOptions = append(gatewayOptions, WithGatewayLogCacheDialOpts(
			grpc.WithInsecure(),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*8)),
		),
		)

	}

	gateway := NewGateway(cfg.LogCacheAddr, cfg.Addr, gatewayOptions...)

	gateway.Start()
}
