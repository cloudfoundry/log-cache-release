package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.cloudfoundry.org/go-loggregator/metrics"

	"code.cloudfoundry.org/go-envstruct"
	. "code.cloudfoundry.org/log-cache/internal/cache"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting Log Cache...")
	defer log.Print("Closing Log Cache.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	logger := log.New(os.Stderr, "", log.LstdFlags)

	metricServerOption := metrics.WithServer(int(cfg.MetricsServer.Port))

	if cfg.MetricsServer.CAFile != "" {
		metricServerOption = metrics.WithTLSServer(
			int(cfg.MetricsServer.Port),
			cfg.MetricsServer.CertFile,
			cfg.MetricsServer.KeyFile,
			cfg.MetricsServer.CAFile,
		)
	}

	m := metrics.NewRegistry(
		logger,
		metricServerOption,
	)

	uptimeFn := m.NewGauge(
		"log_cache_uptime",
		metrics.WithHelpText("Time since log cache started."),
		metrics.WithMetricTags(map[string]string{
			"unit": "seconds",
		}),
	)

	t := time.NewTicker(time.Second)
	go func(start time.Time) {
		for range t.C {
			uptimeFn.Set(float64(time.Since(start) / time.Second))
		}
	}(time.Now())

	cache := New(
		m,
		logger,
		WithAddr(cfg.Addr),
		WithMemoryLimit(float64(cfg.MemoryLimit)),
		WithMaxPerSource(cfg.MaxPerSource),
		WithQueryTimeout(cfg.QueryTimeout),
		WithClustered(
			cfg.NodeIndex,
			cfg.NodeAddrs,
			grpc.WithTransportCredentials(
				cfg.TLS.Credentials("log-cache"),
			),
		),
		WithServerOpts(grpc.Creds(cfg.TLS.Credentials("log-cache"))),
	)

	cache.Start()
	waitForTermination()
}

func waitForTermination() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
