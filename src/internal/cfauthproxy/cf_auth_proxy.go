package cfauthproxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"code.cloudfoundry.org/log-cache/internal/auth"
	sharedtls "code.cloudfoundry.org/log-cache/internal/tls"
	"github.com/gorilla/mux"
)

type CFAuthProxy struct {
	tlsDisabled  bool
	blockOnStart bool
	ln           net.Listener

	gatewayURL      *url.URL
	metricsURL      *url.URL
	addr            string
	certPath        string
	keyPath         string
	proxyCACertPool *x509.CertPool

	authMiddleware   func(http.Handler) http.Handler
	accessMiddleware func(http.Handler) *auth.AccessHandler
}

func NewCFAuthProxy(gatewayAddr, metricAddr, addr string, opts ...CFAuthProxyOption) *CFAuthProxy {
	gatewayURL, err := url.Parse(gatewayAddr)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse gateway address: %s", err))
	}

	metricURL, err := url.Parse(metricAddr)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse gateway address: %s", err))
	}

	fmt.Println("************************")
	fmt.Println(metricURL)
	fmt.Println("************************")

	p := &CFAuthProxy{
		gatewayURL: gatewayURL,
		metricsURL: metricURL,
		addr:       addr,
		authMiddleware: func(h http.Handler) http.Handler {
			return h
		},
		accessMiddleware: auth.NewNullAccessMiddleware(),
	}

	for _, o := range opts {
		o(p)
	}

	return p
}

// CFAuthProxyOption configures a CFAuthProxy
type CFAuthProxyOption func(*CFAuthProxy)

// WithCFAuthProxyBlock returns a CFAuthProxyOption that determines if Start
// launches a go-routine or not. It defaults to launching a go-routine. If
// this is set, start will block on serving the HTTP endpoint.
func WithCFAuthProxyBlock() CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.blockOnStart = true
	}
}

func WithCFAuthProxyTLSServer(certPath, keyPath string) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.keyPath = keyPath
		p.certPath = certPath
	}
}

// WithAuthMiddleware returns a CFAuthProxyOption that sets the CFAuthProxy's
// authentication and authorization middleware.
func WithAuthMiddleware(authMiddleware func(http.Handler) http.Handler) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.authMiddleware = authMiddleware
	}
}

func WithAccessMiddleware(accessMiddleware func(http.Handler) *auth.AccessHandler) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.accessMiddleware = accessMiddleware
	}
}

// WithCFAuthProxyTLSDisabled returns a CFAuthProxyOption that sets the CFAuthProxy
// to accept insecure plain text communication
func WithCFAuthProxyTLSDisabled() CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.tlsDisabled = true
	}
}

// WithCFAuthProxyCACertPool returns a CFAuthProxyOption that sets the
// CFAuthProxy CA Cert pool. Otherwise, the CFAuthProxy communicates with the
// gateway in plain text.
func WithCFAuthProxyCACertPool(certPool *x509.CertPool) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.proxyCACertPool = certPool
	}
}

// Start starts the HTTP listener and serves the HTTP server. If the
// CFAuthProxy was initialized with the WithCFAuthProxyBlock option this
// method will block.
func (p *CFAuthProxy) Start() {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	p.ln = ln
	if p.blockOnStart {
		p.startServer()
	}

	go func() {
		p.startServer()
	}()
}

func (p *CFAuthProxy) startServer() {
	h := mux.NewRouter()
	h.HandleFunc("/api/v1/query", p.metricReverseProxy().ServeHTTP)
	h.PathPrefix("/").HandlerFunc(p.accessMiddleware(p.authMiddleware(p.logReverseProxy())).ServeHTTP)

	fmt.Println("I Should be proxying back to the metrics server")

	server := http.Server{
		Handler:   h,
		TLSConfig: sharedtls.NewBaseTLSConfig(),
	}

	if p.tlsDisabled {
		log.Fatal(server.Serve(p.ln))
	} else {
		log.Fatal(server.ServeTLS(p.ln, p.certPath, p.keyPath))
	}
}

// Addr returns the listener address. This must be called after calling Start.
func (p *CFAuthProxy) Addr() string {
	return p.ln.Addr().String()
}

func (p *CFAuthProxy) logReverseProxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(p.gatewayURL)

	if p.proxyCACertPool != nil {
		proxy.Transport = NewTransportWithRootCA(p.proxyCACertPool)
	}

	return proxy
}

func (p *CFAuthProxy) metricReverseProxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(p.metricsURL)
	return proxy
}

func NewTransportWithRootCA(rootCACertPool *x509.CertPool) *http.Transport {
	// Aside from the Root CA for the gateway, these values are defaults
	// from Golang's http.DefaultTransport
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs: rootCACertPool,
		},
	}
}
