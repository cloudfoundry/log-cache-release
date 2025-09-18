package main

import (
	"fmt"
	"log"
	"net/http"

	//nolint:gosec
	_ "net/http/pprof"

	"os"
	"time"

	"code.cloudfoundry.org/go-envstruct"
	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/log-cache/internal/nozzle"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"code.cloudfoundry.org/log-cache/internal/syslog"
	"code.cloudfoundry.org/tlsconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
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

	log.Print("Starting Syslog Server...")
	defer log.Print("Closing Syslog Server.")

	err = envstruct.WriteReport(cfg)
	if err != nil {
		log.Printf("Failed to print a report of the from environment: %s\n", err)
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
		loggr,
		metricServerOption,
	)
	if cfg.MetricsServer.DebugMetrics {
		m.RegisterDebugMetrics()
		pprofServer := &http.Server{
			Addr:              fmt.Sprintf("127.0.0.1:%d", cfg.MetricsServer.PprofPort),
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 2 * time.Second,
		}
		go func() { loggr.Println("PPROF SERVER STOPPED " + pprofServer.ListenAndServe().Error()) }()
	}

	serverOptions := []syslog.ServerOption{
		syslog.WithServerPort(cfg.SyslogPort),
		syslog.WithIdleTimeout(cfg.SyslogIdleTimeout),
		syslog.WithServerMaxMessageLength(cfg.SyslogMaxMessageLength),
		syslog.WithServerTrimMessageWhitespace(cfg.SyslogTrimMessageWhitespace),
	}
	if cfg.SyslogTLSCertPath != "" || cfg.SyslogTLSKeyPath != "" {
		serverOptions = append(serverOptions, syslog.WithServerTLS(cfg.SyslogTLSCertPath, cfg.SyslogTLSKeyPath))
	}
	if cfg.SyslogClientTrustedCAFile != "" {
		serverOptions = append(serverOptions, syslog.WithSyslogClientCA(cfg.SyslogClientTrustedCAFile))
	}

	if cfg.SyslogNonTransparentFraming {
		serverOptions = append(serverOptions, syslog.WithNonTransparentFraming())
	}

	server := syslog.NewServer(
		loggr,
		m,
		serverOptions...,
	)

	go server.Start()

	nozzleOptions := []nozzle.NozzleOption{}
	if cfg.LogCacheTLS.HasAnyCredential() {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(cfg.LogCacheTLS.CertPath, cfg.LogCacheTLS.KeyPath),
		).Client(
			tlsconfig.WithAuthorityFromFile(cfg.LogCacheTLS.CAPath),
			tlsconfig.WithServerName("log-cache"),
		)
		if err != nil {
			panic(err)
		}
		nozzleOptions = append(nozzleOptions, nozzle.WithDialOpts(grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))))
	} else {
		nozzleOptions = append(nozzleOptions, nozzle.WithDialOpts(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}

	noz := nozzle.NewNozzle(
		server,
		cfg.LogCacheAddr,
		m,
		loggr,
		nozzleOptions...,
	)

	noz.Start()
}
