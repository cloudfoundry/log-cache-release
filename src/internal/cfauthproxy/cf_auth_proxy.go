package cfauthproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/log-cache/internal/auth"
	sharedtls "code.cloudfoundry.org/log-cache/internal/tls"
)

type CFAuthProxy struct {
	tlsDisabled bool
	ln          net.Listener
	server      http.Server
	mu          sync.Mutex

	gatewayURL        *url.URL
	addr              string
	certPath          string
	keyPath           string
	proxyCACertPool   *x509.CertPool
	readinessInterval time.Duration

	authMiddleware   func(http.Handler) http.Handler
	accessMiddleware func(http.Handler) *auth.AccessHandler
	promMiddleware   func(http.Handler) http.Handler
}

func NewCFAuthProxy(gatewayAddr, addr string, opts ...CFAuthProxyOption) *CFAuthProxy {
	gatewayURL, err := url.Parse(gatewayAddr)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse gateway address: %s", err))
	}

	p := &CFAuthProxy{
		gatewayURL: gatewayURL,
		addr:       addr,
		authMiddleware: func(h http.Handler) http.Handler {
			return h
		},
		promMiddleware: func(h http.Handler) http.Handler {
			return h
		},
		accessMiddleware:  auth.NewNullAccessMiddleware(),
		readinessInterval: 10 * time.Second,
	}

	for _, o := range opts {
		o(p)
	}

	return p
}

// CFAuthProxyOption configures a CFAuthProxy
type CFAuthProxyOption func(*CFAuthProxy)

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

func WithPromMiddleware(promMiddleware func(http.Handler) http.Handler) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.promMiddleware = promMiddleware
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

func WithCFAuthProxyReadyCheckInterval(interval time.Duration) CFAuthProxyOption {
	return func(p *CFAuthProxy) {
		p.readinessInterval = interval
	}
}

// CFAuthProxy was initialized with the WithCFAuthProxyBlock option this
// method will block.
func (p *CFAuthProxy) Start(readyChecker func() error) {
	for err := readyChecker(); err != nil; err = readyChecker() {
		log.Printf("Not ready to start: %s", err)
		time.Sleep(p.readinessInterval)
	}

	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		log.Fatalf("failed to start listener: %s", err)
	}

	server := http.Server{
		Handler:   p.accessMiddleware(p.promMiddleware(p.authMiddleware(p.reverseProxy()))),
		TLSConfig: sharedtls.NewBaseTLSConfig(),
	}

	p.mu.Lock()
	p.ln = ln
	p.server = server
	p.mu.Unlock()

	if p.tlsDisabled {
		log.Fatal(server.Serve(ln))
	} else {
		log.Fatal(server.ServeTLS(ln, p.certPath, p.keyPath))
	}
}

func (p *CFAuthProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	p.server.Shutdown(ctx)
}

// Addr returns the listener address. This must be called after calling Start.
func (p *CFAuthProxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ln != nil {
		return p.ln.Addr().String()
	}
	return ""
}

func (p *CFAuthProxy) reverseProxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(p.gatewayURL)

	if p.proxyCACertPool != nil {
		proxy.Transport = NewTransportWithRootCA(p.proxyCACertPool)
	}

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
