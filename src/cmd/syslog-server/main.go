package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"time"

	"code.cloudfoundry.org/go-loggregator/metrics"
	"code.cloudfoundry.org/log-cache/internal/routing"
	"code.cloudfoundry.org/log-cache/internal/syslog"
	"code.cloudfoundry.org/log-cache/pkg/rpc/logcache_v1"

	"code.cloudfoundry.org/go-envstruct"
	"google.golang.org/grpc"
)

const (
	BATCH_FLUSH_INTERVAL = 500 * time.Millisecond
	BATCH_CHANNEL_SIZE   = 512
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Print("Starting Syslog Server...")
	defer log.Print("Closing Syslog Server.")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("invalid configuration: %s", err)
	}

	envstruct.WriteReport(cfg)

	loggr := log.New(os.Stderr, "[LOGGR] ", log.LstdFlags)

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
		loggr,
		metricServerOption,
	)

	conn, err := grpc.Dial(
		cfg.LogCacheAddr,
		grpc.WithTransportCredentials(
			cfg.LogCacheTLS.Credentials("log-cache"),
		),
	)

	client := logcache_v1.NewIngressClient(conn)

	egressDropped := m.NewCounter(
		"egress_dropped",
		metrics.WithHelpText("Total number of envelopes dropped while sending to log cache."),
	)
	sendFailures := m.NewCounter(
		"log_cache_send_failure",
		metrics.WithHelpText("Total number of envelope batches failed to send to log cache."),
		metrics.WithMetricTags(map[string]string{"sender": "batched_ingress_client", "source": "syslog_server"}),
	)
	logCacheClient := routing.NewBatchedIngressClient(
		BATCH_CHANNEL_SIZE,
		BATCH_FLUSH_INTERVAL,
		client,
		egressDropped,
		sendFailures,
		loggr,
		routing.WithLocalOnlyDisabled,
	)
	server := syslog.NewServer(
		loggr,
		logCacheClient,
		m,
		cfg.SyslogTLSCertPath,
		cfg.SyslogTLSKeyPath,
		syslog.WithServerPort(cfg.SyslogPort),
		syslog.WithIdleTimeout(cfg.SyslogIdleTimeout),
	)

	server.Start()
}
