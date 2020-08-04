package main

import (
	"log"
	_ "net/http/pprof"
	"os"

	metrics "code.cloudfoundry.org/go-metric-registry"
	. "code.cloudfoundry.org/log-cache/internal/nozzle"
	"code.cloudfoundry.org/log-cache/internal/syslog"

	"code.cloudfoundry.org/go-envstruct"
	"google.golang.org/grpc"
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

	server := syslog.NewServer(
		loggr,
		m,
		cfg.SyslogTLSCertPath,
		cfg.SyslogTLSKeyPath,
		syslog.WithServerPort(cfg.SyslogPort),
		syslog.WithIdleTimeout(cfg.SyslogIdleTimeout),
	)

	go server.Start()

	nozzle := NewNozzle(
		server,
		cfg.LogCacheAddr,
		m,
		loggr,
		WithDialOpts(
			grpc.WithTransportCredentials(
				cfg.LogCacheTLS.Credentials("log-cache"),
			),
		),
	)

	nozzle.Start()
}
