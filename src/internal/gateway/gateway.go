package gateway

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/shirou/gopsutil/v3/host"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
	logcacheMarshaler "code.cloudfoundry.org/log-cache/pkg/marshaler"
)

// Gateway provides a RESTful API into LogCache's gRPC API.
type Gateway struct {
	logCacheAddr    string
	logCacheVersion string
	uptimeFn        func() int64

	gatewayAddr      string
	lis              net.Listener
	blockOnStart     bool
	logCacheDialOpts []grpc.DialOption
	certPath         string
	keyPath          string
}

// NewGateway creates a new Gateway. It will listen on the gatewayAddr and
// submit requests via gRPC to the LogCache on logCacheAddr. Start() must be
// invoked before using the Gateway.
func NewGateway(logCacheAddr, gatewayAddr string, opts ...GatewayOption) *Gateway {
	g := &Gateway{
		logCacheAddr: logCacheAddr,
		gatewayAddr:  gatewayAddr,
		uptimeFn:     uptimeInSeconds,
	}

	for _, o := range opts {
		o(g)
	}

	return g
}

// GatewayOption configures a Gateway.
type GatewayOption func(*Gateway)

// WithGatewayBlock returns a GatewayOption that determines if Start launches
// a go-routine or not. It defaults to launching a go-routine. If this is set,
// start will block on serving the HTTP endpoint.
func WithGatewayBlock() GatewayOption {
	return func(g *Gateway) {
		g.blockOnStart = true
	}
}

// WithGatewayLogCacheDialOpts returns a GatewayOption that sets grpc.DialOptions on the
// log-cache dial
func WithGatewayLogCacheDialOpts(opts ...grpc.DialOption) GatewayOption {
	return func(g *Gateway) {
		g.logCacheDialOpts = opts
	}
}

// WithGatewayVersion returns a GatewayOption that sets the log-cache
// version returned by the info endpoint.
func WithGatewayVersion(version string) GatewayOption {
	return func(g *Gateway) {
		g.logCacheVersion = version
	}
}

// WithGatewayVMUptimeFn returns a GatewayOption that sets the VM
// uptime function.
func WithGatewayVMUptimeFn(uptimeFn func() int64) GatewayOption {
	return func(g *Gateway) {
		g.uptimeFn = uptimeFn
	}
}

func WithGatewayTLSServer(certPath, keyPath string) GatewayOption {
	return func(g *Gateway) {
		g.keyPath = keyPath
		g.certPath = certPath
	}
}

// Start starts the gateway to start receiving and forwarding requests. It
// does not block unless WithGatewayBlock was set.
func (g *Gateway) Start() {
	slog.Info("Starting server", "address", g.gatewayAddr)
	lis, err := net.Listen("tcp", g.gatewayAddr)
	if err != nil {
		panic(err)
	}
	g.lis = lis

	if g.blockOnStart {
		g.listenAndServe()
		return
	}

	go g.listenAndServe()
}

// Addr returns the address the gateway is listening on. Start must be called
// first.
func (g *Gateway) Addr() string {
	return g.lis.Addr().String()
}

func (g *Gateway) listenAndServe() {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard, logcacheMarshaler.NewPromqlMarshaler(&runtime.JSONPb{MarshalOptions: protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}}),
		),
		runtime.WithErrorHandler(g.httpErrorHandler),
	)

	conn, err := grpc.NewClient(g.logCacheAddr, g.logCacheDialOpts...)
	if err != nil {
		panic(err)
	}

	err = logcache_v1.RegisterEgressHandlerClient(
		context.Background(),
		mux,
		logcache_v1.NewEgressClient(conn),
	)
	if err != nil {
		panic(err)
	}

	err = logcache_v1.RegisterPromQLQuerierHandlerClient(
		context.Background(),
		mux,
		logcache_v1.NewPromQLQuerierClient(conn),
	)
	if err != nil {
		panic(err)
	}

	topLevelMux := http.NewServeMux()
	topLevelMux.HandleFunc("/api/v1/info", g.handleInfoEndpoint)
	topLevelMux.Handle("/", mux)

	server := &http.Server{
		Handler:           topLevelMux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	if g.certPath != "" || g.keyPath != "" {
		if err := server.ServeTLS(g.lis, g.certPath, g.keyPath); err != nil {
			panic(err)
		}
	} else {
		if err := server.Serve(g.lis); err != nil {
			panic(err)
		}
	}
}

func (g *Gateway) handleInfoEndpoint(w http.ResponseWriter, r *http.Request) {
	// _, err := w.Write([]byte(fmt.Sprintf(`{"version":"%s","vm_uptime":"%d"}`+"\n", g.logCacheVersion, g.uptimeFn())))
	// if err != nil {
	// 	slog.Error("Failed to send result for the info endpoint", "error", err)
	// }
	w.Write([]byte("success\n"))
}

func uptimeInSeconds() int64 {
	hostStats, _ := host.Info()
	return int64(hostStats.Uptime)
}

type errorBody struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (g *Gateway) httpErrorHandler(
	ctx context.Context,
	mux *runtime.ServeMux,
	marshaler runtime.Marshaler,
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	if r.URL.Path != "/api/v1/query" && r.URL.Path != "/api/v1/query_range" {
		runtime.DefaultHTTPErrorHandler(ctx, mux, marshaler, w, r, err)
		return
	}

	const fallback = `{"error": "failed to marshal error message"}`

	w.Header().Del("Trailer")
	w.Header().Set("Content-Type", marshaler.ContentType(nil))

	body := &errorBody{
		Status:    "error",
		ErrorType: "internal",
		Error:     status.Convert(err).Message(),
	}

	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		slog.Error("Failed to marshal error message", "error", merr)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			slog.Error("Failed to write response", "error", err)
		}
		return
	}

	w.WriteHeader(runtime.HTTPStatusFromCode(status.Code(err)))
	if _, err := w.Write(buf); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}
