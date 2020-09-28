package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"

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
		logger,
		metricServerOption,
	)

	uptimeFn := m.NewGauge(
		"log_cache_uptime",
		"Time since log cache started.",
		metrics.WithMetricLabels(map[string]string{
			"unit": "seconds",
		}),
	)

	t := time.NewTicker(time.Second)
	go func(start time.Time) {
		for range t.C {
			uptimeFn.Set(float64(time.Since(start) / time.Second))
		}
	}(time.Now())

	logCacheOptions := []LogCacheOption{
		WithAddr(cfg.Addr),
		WithMemoryLimitPercent(float64(cfg.MemoryLimitPercent)),
		WithMemoryLimit(cfg.MemoryLimit),
		WithMaxPerSource(cfg.MaxPerSource),
		WithQueryTimeout(cfg.QueryTimeout),
	}
	var transport grpc.DialOption
	if cfg.TLS.HasAnyCredential() {
		transport = grpc.WithTransportCredentials(
			cfg.TLS.Credentials("log-cache"),
		)
		logCacheOptions = append(logCacheOptions, WithServerOpts(grpc.Creds(cfg.TLS.Credentials("log-cache"))))
	} else {
		transport = grpc.WithInsecure()
	}
	logCacheOptions = append(logCacheOptions, WithClustered(
		cfg.NodeIndex,
		cfg.NodeAddrs,
		transport,
	))

	cache := New(
		m,
		logger,
		logCacheOptions...,
	)

	cache.Start()
	waitForTermination()
}

func waitForTermination() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
