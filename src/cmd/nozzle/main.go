package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	metrics "code.cloudfoundry.org/go-metric-registry"

	envstruct "code.cloudfoundry.org/go-envstruct"
	. "code.cloudfoundry.org/log-cache/internal/nozzle"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"google.golang.org/grpc"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

func main() {
	var loggr *log.Logger

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	if cfg.UseRFC339 {
		loggr = log.New(new(plumbing.LogWriter), "[LOGGR] ", 0)
		log.SetOutput(new(plumbing.LogWriter))
		log.SetFlags(0)
	} else {
		loggr = log.New(os.Stderr, "[LOGGR] ", log.LstdFlags)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	log.Print("Starting LogCache Nozzle...")
	defer log.Print("Closing LogCache Nozzle.")

	envstruct.WriteReport(cfg)

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
		loggr,
		metricServerOption,
	)

	dropped := m.NewCounter(
		"nozzle_dropped",
		"Total number of envelopes dropped.",
	)

	tlsCfg, err := loggregator.NewEgressTLSConfig(
		cfg.LogsProviderTLS.LogProviderCA,
		cfg.LogsProviderTLS.LogProviderCert,
		cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		log.Fatalf("invalid LogsProviderTLS configuration: %s", err)
	}

	streamConnector := loggregator.NewEnvelopeStreamConnector(
		cfg.LogProviderAddr,
		tlsCfg,
		loggregator.WithEnvelopeStreamLogger(loggr),
		loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
			loggr.Printf("dropped %d envelope batches", missed)
			dropped.Add(float64(missed))
		}),
	)

	nozzleOptions := []NozzleOption{
		WithDialOpts(
			grpc.WithTransportCredentials(
				cfg.LogCacheTLS.Credentials("log-cache"),
			),
		),
		WithSelectors(cfg.Selectors...),
		WithShardID(cfg.ShardId),
	}

	nozzle := NewNozzle(
		streamConnector,
		cfg.LogCacheAddr,
		m,
		loggr,
		nozzleOptions...,
	)

	go nozzle.Start()
	waitForTermination()
}

func waitForTermination() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
