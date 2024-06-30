package integrationfakes

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	logcache "code.cloudfoundry.org/go-log-cache/v3/rpc/logcache_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// FakeLogCache is a fake implementation of log-cache.
type FakeLogCache struct {
	port    int          // Port to listen on.
	addr    string       // Address of the net listener.
	c       *tls.Config  // TLS config to apply to gRPC; no TLS if nil.
	s       *grpc.Server // gRPC server responding to Log Cache gRPC requests.
	serveCh chan error   // Channel to catch errors when the serve errors from the gRPC server.

	readMu       sync.Mutex              // Mutex to prevent race conditions with FakeLogCache.Read().
	readRequests []*logcache.ReadRequest // Slice of requests made to FakeLogCache.Read().
	readResponse *logcache.ReadResponse
	readErr      error

	logcache.UnimplementedEgressServer
	logcache.UnimplementedIngressServer
	logcache.UnimplementedPromQLQuerierServer
}

// NewFakeLogCache creates a new instance of FakeLogCache with the specified
// port and TLS configuration.
func NewFakeLogCache(port int, c *tls.Config) *FakeLogCache {
	return &FakeLogCache{
		port:    port,
		c:       c,
		serveCh: make(chan error),
	}
}

// Start attempts to claim a net listener on FakeLogCache's port and
// start a log-cache gRPC server in a separate goroutine. The server uses
// FakeLogCache's TLS config if it was provided. This is a non-blocking
// operation and returns an error if it fails.
//
// If FakeLogCache is started, it must be stopped with Stop().
func (f *FakeLogCache) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", f.port))
	if err != nil {
		return err
	}
	f.addr = lis.Addr().String()

	var opts []grpc.ServerOption
	if f.c != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(f.c)))
	}
	f.s = grpc.NewServer(opts...)

	logcache.RegisterEgressServer(f.s, f)
	logcache.RegisterIngressServer(f.s, f)
	logcache.RegisterPromQLQuerierServer(f.s, f)

	go func() {
		f.serveCh <- f.s.Serve(lis)
	}()

	return nil
}

// Address returns the address of the FakeLogCache.
func (f *FakeLogCache) Address() string {
	return f.addr
}

// Read accepts incoming gRPC requests to read from Log Cache, captures the
// requests and returns a fake response.
func (f *FakeLogCache) Read(ctx context.Context, req *logcache.ReadRequest) (*logcache.ReadResponse, error) {
	fmt.Printf("Read: %#v\n", req)
	f.readMu.Lock()
	defer f.readMu.Unlock()
	f.readRequests = append(f.readRequests, req)
	return f.readResponse, f.readErr
}

func (f *FakeLogCache) ReadRequests() []*logcache.ReadRequest {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	return f.readRequests
}

// Stop tells the FakeLogCache server to stop and waits for it to shutdown.
func (f *FakeLogCache) Stop() error {
	f.s.Stop()
	return <-f.serveCh
}
