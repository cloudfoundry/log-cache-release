package integrationfakes

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	logcache "code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// FakeLogCache is a fake implementation of log-cache.
type FakeLogCache struct {
	port int
	addr string
	c    *tls.Config
	s    *grpc.Server

	serveCh chan error

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

	f.s = grpc.NewServer()
	if f.c != nil {
		f.s = grpc.NewServer(grpc.Creds(credentials.NewTLS(f.c)))
	}

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

func (f *FakeLogCache) Read(ctx context.Context, req *logcache.ReadRequest) (*logcache.ReadResponse, error) {
	fmt.Printf("Read: %#v\n", req)
	return nil, nil
}

// Stop tells the FakeLogCache server to stop and waits for it to shutdown.
func (f *FakeLogCache) Stop() error {
	f.s.Stop()
	return <-f.serveCh
}
