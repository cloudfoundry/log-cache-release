package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"code.cloudfoundry.org/go-loggregator/metrics"

	envstruct "code.cloudfoundry.org/go-envstruct"
	. "code.cloudfoundry.org/log-cache/internal/nozzle"
	"google.golang.org/grpc"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

type tokenAttacher struct {
	token string
}

func (a *tokenAttacher) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", a.token)
	return http.DefaultClient.Do(req)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting LogCache Nozzle...")
	defer log.Print("Closing LogCache Nozzle.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	tlsCfg, err := loggregator.NewEgressTLSConfig(
		cfg.LogsProviderTLS.LogProviderCA,
		cfg.LogsProviderTLS.LogProviderCert,
		cfg.LogsProviderTLS.LogProviderKey,
	)
	if err != nil {
		log.Fatalf("invalid LogsProviderTLS configuration: %s", err)
	}

	loggr := log.New(os.Stderr, "[LOGGR] ", log.LstdFlags)
	m := metrics.NewRegistry(loggr)

	dropped := m.NewCounter(
		"nozzle_dropped",
		metrics.WithHelpText("Total number of envelopes dropped."),
	)

	var streamConnector StreamConnector
	if !cfg.USE_GATEWAY {
		streamConnector = loggregator.NewEnvelopeStreamConnector(
			cfg.LogProviderAddr,
			tlsCfg,
			loggregator.WithEnvelopeStreamLogger(loggr),
			loggregator.WithEnvelopeStreamBuffer(10000, func(missed int) {
				loggr.Printf("dropped %d envelope batches", missed)
				dropped.Add(float64(missed))
			}),
		)
	} else {
		streamConnector = loggregator.NewRLPGatewayClient(
			cfg.GATEWAY_ADDR,
			loggregator.WithRLPGatewayClientLogger(loggr),
			loggregator.WithRLPGatewayHTTPClient(&tokenAttacher{
				token: cfg.GATEWAY_TOKEN,
			}),
		)
	}

	nozzle := NewNozzle(
		streamConnector,
		cfg.LogCacheAddr,
		cfg.ShardId,
		m,
		loggr,
		WithDialOpts(
			grpc.WithTransportCredentials(
				cfg.LogCacheTLS.Credentials("log-cache"),
			),
		),
		WithSelectors(cfg.Selectors...),
	)

	go nozzle.Start()

	// health endpoints (pprof and prometheus)
	log.Printf("Health: %s", http.ListenAndServe(fmt.Sprintf("localhost:%d", cfg.HealthPort), nil))
}
