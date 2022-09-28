package gateway_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	rpc "code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	. "code.cloudfoundry.org/log-cache/internal/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"code.cloudfoundry.org/log-cache/internal/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func gatewayTestSetup() (*Gateway, *testing.SpyLogCache) {
	spyLogCache := testing.NewSpyLogCache(nil)
	logCacheAddr := spyLogCache.Start()

	gw := NewGateway(
		logCacheAddr,
		"localhost:0",
		WithGatewayVersion("1.2.3"),
		WithGatewayVMUptimeFn(testing.StubUptimeFn),
		WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	gw.Start()

	return gw, spyLogCache
}

func tlsGatewayTestSetup() (*Gateway, *testing.SpyLogCache) {
	tlsConfig, err := testing.NewTLSConfig(
		testing.LogCacheTestCerts.CA(),
		testing.LogCacheTestCerts.Cert("log-cache"),
		testing.LogCacheTestCerts.Key("log-cache"),
		"log-cache",
	)
	Expect(err).ToNot(HaveOccurred())

	spyLogCache := testing.NewSpyLogCache(tlsConfig)
	logCacheAddr := spyLogCache.Start()

	localHostCerts := testing.GenerateCerts("localhost-ca")
	gw := NewGateway(
		logCacheAddr,
		"localhost:0",
		WithGatewayTLSServer(localHostCerts.Cert("localhost"), localHostCerts.Key("localhost")),
		WithGatewayLogCacheDialOpts(
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		),
		WithGatewayVersion("1.2.3"),
		WithGatewayVMUptimeFn(testing.StubUptimeFn),
	)
	gw.Start()

	return gw, spyLogCache
}

var _ = Describe("Gateway", func() {
	DescribeTable("upgrades HTTPS requests for LogCache into gRPC requests", func(pathSourceID, expectedSourceID string) {
		gw, spyLogCache := tlsGatewayTestSetup()
		path := fmt.Sprintf("api/v1/read/%s?start_time=99&end_time=101&limit=103&envelope_types=LOG&envelope_types=GAUGE", pathSourceID)
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyLogCache.GetReadRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].SourceId).To(Equal(expectedSourceID))
		Expect(reqs[0].StartTime).To(Equal(int64(99)))
		Expect(reqs[0].EndTime).To(Equal(int64(101)))
		Expect(reqs[0].Limit).To(Equal(int64(103)))
		Expect(reqs[0].EnvelopeTypes).To(ConsistOf(rpc.EnvelopeType_LOG, rpc.EnvelopeType_GAUGE))
	},
		Entry("URL encoded", "some-source%2Fid", "some-source/id"),
		Entry("with slash", "some-source/id", "some-source/id"),
		Entry("with dash", "some-source-id", "some-source-id"),
	)

	It("adds newlines to the end of HTTPS responses", func() {
		gw, _ := tlsGatewayTestSetup()
		path := `api/v1/meta`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		respBytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(respBytes)).To(MatchRegexp(`\n$`))
	})

	It("upgrades HTTPS requests for instant queries via PromQLQuerier GETs into gRPC requests", func() {
		gw, spyLogCache := tlsGatewayTestSetup()
		path := `api/v1/query?query=metric{source_id="some-id"}&time=1234.000`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyLogCache.GetQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Query).To(Equal(`metric{source_id="some-id"}`))
		Expect(reqs[0].Time).To(Equal("1234.000"))
	})

	It("can operate without tls", func() {
		gw, spyLogCache := gatewayTestSetup()
		path := `api/v1/query?query=metric{source_id="some-id"}&time=1234.000`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyLogCache.GetQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Query).To(Equal(`metric{source_id="some-id"}`))
		Expect(reqs[0].Time).To(Equal("1234.000"))
	})
	It("upgrades HTTPS requests for range queries via PromQLQuerier GETs into gRPC requests", func() {
		gw, spyLogCache := tlsGatewayTestSetup()
		path := `api/v1/query_range?query=metric{source_id="some-id"}&start=1234.000&end=5678.000&step=30s`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		reqs := spyLogCache.GetRangeQueryRequests()
		Expect(reqs).To(HaveLen(1))
		Expect(reqs[0].Query).To(Equal(`metric{source_id="some-id"}`))
		Expect(reqs[0].Start).To(Equal("1234.000"))
		Expect(reqs[0].End).To(Equal("5678.000"))
		Expect(reqs[0].Step).To(Equal("30s"))
	})

	It("outputs json with zero-value points and correct Prometheus API fields", func() {
		gw, spyLogCache := tlsGatewayTestSetup()
		path := `api/v1/query?query=metric{source_id="some-id"}&time=1234`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		spyLogCache.SetValue(0)

		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		body, _ := io.ReadAll(resp.Body)
		Expect(body).To(MatchJSON(`{"status":"success","data":{"resultType":"scalar","result":[99,"0"]}}`))
	})

	It("returns version information from an info endpoint", func() {
		gw, _ := tlsGatewayTestSetup()
		path := `api/v1/info`
		URL := fmt.Sprintf("%s/%s", gw.Addr(), path)
		resp, err := makeTLSReq(URL)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		respBytes, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(respBytes).To(MatchJSON(
			`{
			"version":"1.2.3",
			"vm_uptime":"789"
		}`))
		Expect(strings.HasSuffix(string(respBytes), "\n")).To(BeTrue())
	})

	It("does not accept unencrypted connections", func() {
		gw, _ := tlsGatewayTestSetup()
		resp, err := makeReq(fmt.Sprintf("%s/api/v1/info", gw.Addr()))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
	})

	Context("errors", func() {
		It("passes through content-type correctly on errors", func() {
			gw, spyLogCache := tlsGatewayTestSetup()
			path := `api/v1/query?query=metric{source_id="some-id"}&time=1234`
			spyLogCache.QueryError = errors.New("expected error")
			URL := fmt.Sprintf("%s/%s", gw.Addr(), path)

			resp, err := makeTLSReq(URL)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(resp.Header).To(HaveKeyWithValue("Content-Type", []string{"application/json"}))
		})

		It("adds necessary fields to match Prometheus API", func() {
			gw, spyLogCache := tlsGatewayTestSetup()
			path := `api/v1/query?query=metric{source_id="some-id"}&time=1234`
			spyLogCache.QueryError = errors.New("expected error")
			URL := fmt.Sprintf("%s/%s", gw.Addr(), path)

			resp, err := makeTLSReq(URL)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			body, _ := io.ReadAll(resp.Body)
			Expect(body).To(MatchJSON(`{
				"status": "error",

				"errorType": "internal",
				"error": "expected error"
			}`))
		})
	})
})

func makeTLSReq(addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s", addr), nil)
	Expect(err).ToNot(HaveOccurred())

	//nolint:gosec
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	return client.Do(req)
}

func makeReq(addr string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s", addr), nil)
	Expect(err).ToNot(HaveOccurred())

	client := &http.Client{}

	return client.Do(req)
}
