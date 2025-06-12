package main

import (
	"fmt"
	"log"
	"net/http"

	//nolint:gosec
	_ "net/http/pprof"

	"os"
	"os/signal"
	"syscall"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/go-envstruct"
	. "code.cloudfoundry.org/log-cache/internal/cache"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var logger *log.Logger

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	if cfg.UseRFC339 {
		logger = log.New(new(plumbing.LogWriter), "", 0)
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		logger = log.New(os.Stderr, "", log.LstdFlags)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	log.Print("Starting Log Cache...")
	defer log.Print("Closing Log Cache.")

	err = envstruct.WriteReport(cfg)
	if err != nil {
		log.Printf("Failed to write a report of the from environment: %s\n", err)
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
		logger,
		metricServerOption,
	)
	if cfg.MetricsServer.DebugMetrics {
		m.RegisterDebugMetrics()
		pprofServer := &http.Server{
			Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.MetricsServer.PprofPort),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 2 * time.Second,
		}
		go func() { logger.Println("PPROF SERVER STOPPED " + pprofServer.ListenAndServe().Error()) }()
	}

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
		WithTruncationInterval(cfg.TruncationInterval),
		WithPrunesPerGC(cfg.PrunesPerGC),
		WithIngressBufferSize(cfg.IngressBufferSize),
		WithIngressBufferReadBatchSize(cfg.IngressBufferReadBatchSize),
		WithIngressBufferReadBatchInterval(cfg.IngressBufferReadBatchInterval),
	}
	var transport grpc.DialOption
	if cfg.TLS.HasAnyCredential() {
		tlsConfigClient, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(cfg.TLS.CertPath, cfg.TLS.KeyPath),
		).Client(
			tlsconfig.WithAuthorityFromFile(cfg.TLS.CAPath),
			tlsconfig.WithServerName("log-cache"),
		)
		if err != nil {
			panic(err)
		}
		transport = grpc.WithTransportCredentials(
			credentials.NewTLS(tlsConfigClient),
		)
		tlsConfigServer, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(cfg.TLS.CertPath, cfg.TLS.KeyPath),
		).Server(
			tlsconfig.WithClientAuthenticationFromFile(cfg.TLS.CAPath),
		)
		if err != nil {
			panic(err)
		}
		logCacheOptions = append(logCacheOptions, WithServerOpts(grpc.Creds(credentials.NewTLS(tlsConfigServer)), grpc.MaxRecvMsgSize(50*1024*1024)))
	} else {
		transport = grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	logCacheOptions = append(logCacheOptions, WithClustered(
		cfg.NodeIndex,
		cfg.NodeAddrs,
		transport,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
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
