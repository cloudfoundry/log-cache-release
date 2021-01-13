package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

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

	tlsCfg, err := loggregator.NewEgressTLSConfig(
		cfg.LogsProviderTLS.LogProviderCA,
		cfg.LogsProviderTLS.LogProviderCert,
		cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		log.Fatalf("invalid LogsProviderTLS configuration: %s", err)
	}

	m := metrics.NewRegistry(loggr)

	dropped := m.NewCounter(
		"nozzle_dropped",
		"Total number of envelopes dropped.",
	)

	streamConnector := loggregator.NewEnvelopeStreamConnector(
		cfg.LogProviderAddr,
		tlsCfg,
		loggregator.WithEnvelopeStreamLogger(loggr),
		loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
			loggr.Printf("dropped %d envelope batches", missed)
			dropped.Add(float64(missed))
		}),
	)

	nozzle := NewNozzle(
		streamConnector,
		cfg.LogCacheAddr,
		m,
		loggr,
		WithDialOpts(
			grpc.WithTransportCredentials(
				cfg.LogCacheTLS.Credentials("log-cache"),
			),
		),
		WithSelectors(cfg.Selectors...),
		WithShardID(cfg.ShardId),
	)

	go nozzle.Start()

	// health endpoints (pprof and prometheus)
	log.Printf("Health: %s", http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil))
}
