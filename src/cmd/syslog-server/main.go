package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"code.cloudfoundry.org/go-envstruct"
	metrics "code.cloudfoundry.org/go-metric-registry"
	. "code.cloudfoundry.org/log-cache/internal/nozzle"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"code.cloudfoundry.org/log-cache/internal/syslog"
	"google.golang.org/grpc"
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
	}
	if cfg.SyslogTLSCertPath != "" || cfg.SyslogTLSKeyPath != "" {
		serverOptions = append(serverOptions, syslog.WithServerTLS(cfg.SyslogTLSCertPath, cfg.SyslogTLSKeyPath))
	}

	server := syslog.NewServer(
		loggr,
		m,
		serverOptions...,
	)

	go server.Start()

	nozzleOptions := []NozzleOption{}
	if cfg.LogCacheTLS.HasAnyCredential() {
		nozzleOptions = append(nozzleOptions, WithDialOpts(grpc.WithTransportCredentials(cfg.LogCacheTLS.Credentials("log-cache"))))
	} else {
		nozzleOptions = append(nozzleOptions, WithDialOpts(grpc.WithInsecure()))
	}

	nozzle := NewNozzle(
		server,
		cfg.LogCacheAddr,
		m,
		loggr,
		nozzleOptions...,
	)

	nozzle.Start()
}
