package integration_test

import (
	"crypto/tls"
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	logcache "code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
)

var pathToGateway string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	path, err := gexec.Build("code.cloudfoundry.org/log-cache/cmd/gateway")
	Expect(err).NotTo(HaveOccurred())
	return []byte(path)
}, func(data []byte) {
	pathToGateway = string(data)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

type FakeLogCache struct {
	addr string
	c    *tls.Config
	s    *grpc.Server

	serveCh chan error

	logcache.UnimplementedEgressServer
	logcache.UnimplementedIngressServer
	logcache.UnimplementedPromQLQuerierServer
}

func NewFakeLogCache(addr string, c *tls.Config) *FakeLogCache {
	return &FakeLogCache{
		addr:    addr,
		c:       c,
		serveCh: make(chan error),
	}
}

func (f *FakeLogCache) Start() {
	lis, err := net.Listen("tcp", f.addr)
	Expect(err).NotTo(HaveOccurred())
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
}

func (f *FakeLogCache) Stop() {
	f.s.Stop()
	Expect(f.serveCh).To(Receive(BeNil()))
}
