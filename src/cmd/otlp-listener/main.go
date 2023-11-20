package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	//nolint:gosec
	_ "net/http/pprof"

	"os"
	"time"

	"code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	metrics "code.cloudfoundry.org/go-metric-registry"
	. "code.cloudfoundry.org/log-cache/internal/nozzle"
	"code.cloudfoundry.org/log-cache/internal/plumbing"
	"code.cloudfoundry.org/tlsconfig"
	prototrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip"
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

	log.Print("Starting OTLP Server...")
	defer log.Print("Closing OTLP Server.")

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

	server := NewOTLPServer()
	go server.Start()

	nozzleOptions := []NozzleOption{}
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
		nozzleOptions = append(nozzleOptions, WithDialOpts(grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))))
	} else {
		nozzleOptions = append(nozzleOptions, WithDialOpts(grpc.WithTransportCredentials(insecure.NewCredentials())))
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

type OTLPServer struct {
	prototrace.UnimplementedTraceServiceServer
	envelopes chan *loggregator_v2.Envelope
}

func NewOTLPServer() *OTLPServer {
	return &OTLPServer{
		envelopes: make(chan *loggregator_v2.Envelope, 100),
	}
}

func (s *OTLPServer) Start() {
	lis, err := net.Listen("tcp", "0.0.0.0:51000")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	prototrace.RegisterTraceServiceServer(grpcServer, s)
	grpcServer.Serve(lis)
}

func (s *OTLPServer) Export(ctx context.Context, req *prototrace.ExportTraceServiceRequest) (*prototrace.ExportTraceServiceResponse, error) {
	fmt.Println("!!!!!!!!!!!!!!!!!!")
	fmt.Printf("%+v\n", req)
	fmt.Println("!!!!!!!!!!!!!!!!!!")

	for _, rs := range req.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				s.envelopes <- &loggregator_v2.Envelope{
					Timestamp: int64(span.EndTimeUnixNano),
					SourceId:  rs.Resource.Attributes[0].Value.GetStringValue(),
					Message: &loggregator_v2.Envelope_Timer{
						Timer: &loggregator_v2.Timer{
							Name:  "http",
							Start: int64(span.StartTimeUnixNano),
							Stop:  int64(span.EndTimeUnixNano),
						},
					},
				}

			}
		}
	}

	//resource_spans:{resource:{attributes:{key:"service.name" value:{string_value:"d42574a8-5102-421e-9e4a-bc74eeecba6d"}}} scope_spans:{scope:{} spans:{trace_id:"4|\xa1\xa5\xf5\x8bLc]\xb1+\xbb\x96\x16\xd6\x14" span_id:"]\xb1+\xbb\x96\x16\xd6\x14" name:"d42574a8-5102-421e-9e4a-bc74eeecba6d" kind:SPAN_KIND_SERVER start_time_unix_nano:1700186276658329120 end_time_unix_nano:1700186276667620350 attributes:{key:"instance_id" value:{string_value:"0"}} attributes:{key:"source_id" value:{string_value:"d42574a8-5102-421e-9e4a-bc74eeecba6d"}} attributes:{key:"process_type" value:{string_value:"web"}} attributes:{key:"process_instance_id" value:{string_value:"c9339f47-e9ef-4eec-5bc3-38dc"}} attributes:{key:"status_code" value:{string_value:"200"}} attributes:{key:"component" value:{string_value:"route-emitter"}} attributes:{key:"app_name" value:{string_value:"dora"}} attributes:{key:"peer_type" value:{string_value:"Server"}} attributes:{key:"organization_id" value:{string_value:"745127e0-17cc-487a-b9d8-82ca898580f0"}} attributes:{key:"method" value:{string_value:"GET"}} attributes:{key:"product" value:{string_value:"Small Footprint VMware Tanzu Application Service"}} attributes:{key:"space_id" value:{string_value:"250525e2-e5c7-4bca-b360-d4d9d33bedb2"}} attributes:{key:"content_length" value:{string_value:"13"}} attributes:{key:"routing_instance_id" value:{string_value:""}} attributes:{key:"user_agent" value:{string_value:"curl/7.81.0"}} attributes:{key:"uri" value:{string_value:"http://dora.apps.herb-322073.cf-app.com/"}} attributes:{key:"job" value:{string_value:"router"}} attributes:{key:"app_id" value:{string_value:"d42574a8-5102-421e-9e4a-bc74eeecba6d"}} attributes:{key:"index" value:{string_value:"8fdb125d-f3d1-4095-8953-5b3343800b65"}} attributes:{key:"origin" value:{string_value:"gorouter"}} attributes:{key:"process_id" value:{string_value:"d42574a8-5102-421e-9e4a-bc74eeecba6d"}} attributes:{key:"deployment" value:{string_value:"cf-0b80a15b89882fe8294e"}} attributes:{key:"remote_address" value:{string_value:"35.224.117.50:34930"}} attributes:{key:"system_domain" value:{string_value:"sys.herb-322073.cf-app.com"}} attributes:{key:"ip" value:{string_value:"10.0.4.9"}} attributes:{key:"space_name" value:{string_value:"s"}} attributes:{key:"request_id" value:{string_value:"347ca1a5-f58b-4c63-5db1-2bbb9616d614"}} attributes:{key:"organization_name" value:{string_value:"o"}} attributes:{key:"forwarded" value:{string_value:""}} attributes:{key:"instance_id" value:{string_value:"0"}} attributes:{key:"process_id" value:{string_value:"d42574a8-5102-421e-9e4a-bc74eeecba6d"}} attributes:{key:"process_instance_id" value:{string_value:"c9339f47-e9ef-4eec-5bc3-38dc"}} status:{code:STATUS_CODE_OK}}}}
	return &prototrace.ExportTraceServiceResponse{}, nil
}

func (s *OTLPServer) Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) loggregator.EnvelopeStream {
	return func() []*loggregator_v2.Envelope {
		select {
		case envelope := <-s.envelopes:
			return []*loggregator_v2.Envelope{envelope}
		case <-ctx.Done():
			return []*loggregator_v2.Envelope{}
		}
	}
}
