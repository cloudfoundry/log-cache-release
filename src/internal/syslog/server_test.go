package syslog_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-metric-registry/testhelpers"
	"code.cloudfoundry.org/log-cache/internal/syslog"
	"code.cloudfoundry.org/log-cache/internal/testing"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const defaultLogMessage = `145 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - [tags@47450 key="value" source_type="actual-source-type"] just a test` + "\n"

func newTlsServerTestSetup(opts ...syslog.ServerOption) (*syslog.Server, *testhelpers.SpyMetricsRegistry, *log.Logger) {
	spyMetrics := testhelpers.NewMetricsRegistry()
	loggr := log.New(GinkgoWriter, "", log.LstdFlags)

	options := []syslog.ServerOption{
		syslog.WithServerTLS(testing.LogCacheTestCerts.Cert("log-cache"), testing.LogCacheTestCerts.Key("log-cache")),
		syslog.WithServerPort(0),
		syslog.WithIdleTimeout(100 * time.Millisecond),
	}
	options = append(options, opts...)

	server := syslog.NewServer(
		loggr,
		spyMetrics,
		options...,
	)

	go server.Start()
	waitForServerToStart(server)
	return server, spyMetrics, loggr
}

func newServerTestSetup() (*syslog.Server, *testhelpers.SpyMetricsRegistry, *log.Logger) {
	spyMetrics := testhelpers.NewMetricsRegistry()
	loggr := log.New(GinkgoWriter, "", log.LstdFlags)

	server := syslog.NewServer(
		loggr,
		spyMetrics,
		syslog.WithIdleTimeout(100*time.Millisecond),
	)

	go server.Start()
	waitForServerToStart(server)
	return server, spyMetrics, loggr
}

var _ = Describe("Syslog", func() {
	It("closes connection after idle timeout", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		go server.Start()
		waitForServerToStart(server)
		defer server.Stop()

		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		readErrs := make(chan error, 1)
		go func() {
			_, err = conn.Read(make([]byte, 1024))
			readErrs <- err
		}()

		Eventually(readErrs).Should(Receive(MatchError(io.EOF)))
	})

	It("keeps the connection open if writes are occurring", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()

		go server.Start()
		waitForServerToStart(server)

		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		Consistently(func() error {
			_, err = fmt.Fprint(conn, defaultLogMessage)
			return err
		}, 1).Should(Succeed())
	})

	It("counts incoming messages", func() {
		server, spyMetrics, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, defaultLogMessage)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, defaultLogMessage)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			return spyMetrics.HasMetric("ingress", nil)
		}).Should(BeTrue())

		Eventually(func() float64 {
			return spyMetrics.GetMetric("ingress", nil).Value()
		}).Should(Equal(2.0))
	})

	It("streams log messages", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, defaultLogMessage)
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				Tags: map[string]string{
					"source_type": "actual-source-type",
					"key":         "value",
				},
				InstanceId: "2",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("just a test"),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			},
		))
	})

	It("max syslog length is configurable", func() {
		server, spyMetrics, _ := newTlsServerTestSetup(syslog.WithServerMaxMessageLength(128))
		defer server.Stop()

		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, "128 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n")
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, "158 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] 123456789012345678901234567890 [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n")
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetric("invalid_ingress", nil).Value()
		}).Should(Equal(1.0))

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags:       map[string]string{},
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"cpu": {Unit: "percentage", Value: 0.23},
						},
					},
				},
			},
		))
	})

	It("streams log messages no tls", func() {
		server, _, _ := newServerTestSetup()
		defer server.Stop()
		conn, err := tcpClientConnection(server.Addr())
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, defaultLogMessage)
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				Tags: map[string]string{
					"source_type": "actual-source-type",
					"key":         "value",
				},
				InstanceId: "2",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Message: &loggregator_v2.Envelope_Log{
					Log: &loggregator_v2.Log{
						Payload: []byte("just a test"),
						Type:    loggregator_v2.Log_OUT,
					},
				},
			},
		))
	})

	It("streams counter messages", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, buildSyslogWithTags(fmt.Sprintf(counterDataFormat, "some-counter", "99", "1"), `key="value"`))
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags: map[string]string{
					"key": "value",
				},
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "some-counter",
						Delta: 1,
						Total: 99,
					},
				},
			},
		))
	})

	It("streams gauge messages", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, buildSyslog(fmt.Sprintf(gaugeDataFormat, "cpu", "0.23", `unit="percentage"`)))
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags:       map[string]string{},
				Message: &loggregator_v2.Envelope_Gauge{
					Gauge: &loggregator_v2.Gauge{
						Metrics: map[string]*loggregator_v2.GaugeValue{
							"cpu": {Unit: "percentage", Value: 0.23},
						},
					},
				},
			},
		))
	})

	It("streams event messages", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, buildSyslog(`event@47450 title="event-title" body="event-body"`))
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags:       map[string]string{},
				Message: &loggregator_v2.Envelope_Event{
					Event: &loggregator_v2.Event{
						Title: "event-title",
						Body:  "event-body",
					},
				},
			},
		))
	})

	It("streams timer messages", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, buildSyslog(`timer@47450 name="some-name" start="10" stop="20"`))
		Expect(err).ToNot(HaveOccurred())

		br := loggregator_v2.EgressBatchRequest{}
		ctx := context.Background()
		Expect(server.Stream(ctx, &br)()).Should(ContainElement(
			&loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags:       map[string]string{},
				Message: &loggregator_v2.Envelope_Timer{
					Timer: &loggregator_v2.Timer{
						Name:  "some-name",
						Start: 10,
						Stop:  20,
					},
				},
			},
		))
	})

	It("can cancel the context of stream", func() {
		server, _, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		_, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		var finished int32
		br := loggregator_v2.EgressBatchRequest{}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			server.Stream(ctx, &br)()
			atomic.AddInt32(&finished, 1)
		}()

		Consistently(func() int32 { return atomic.LoadInt32(&finished) }).Should(Equal(int32(0)))
		cancel()
		Eventually(func() int32 { return atomic.LoadInt32(&finished) }).Should(Equal(int32(1)))
	})

	It("increments invalid message metric when there is an invalid syslog message", func() {
		server, spyMetrics, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		lengthIsWrong := "126 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n"
		_, err = fmt.Fprint(conn, lengthIsWrong)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, "128 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n")
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			return spyMetrics.HasMetric("invalid_ingress", nil)
		}).Should(BeTrue())

		Eventually(func() float64 {
			return spyMetrics.GetMetric("invalid_ingress", nil).Value()
		}).Should(Equal(1.0))

		Eventually(func() bool {
			return spyMetrics.HasMetric("ingress", nil)
		}).Should(BeTrue())

		Eventually(func() float64 {
			return spyMetrics.GetMetric("ingress", nil).Value()
		}).Should(Equal(1.0))
	})

	It("increments invalid message metric when there is missing data", func() {
		server, spyMetrics, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		noAppName := "79 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname - [APP/2] - - just a test\n"
		_, err = fmt.Fprint(conn, noAppName)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("invalid_ingress", nil)
		}).Should(Equal(1.0))

		noProcID := "83 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id - - - just a test\n"
		_, err = fmt.Fprint(conn, noProcID)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("invalid_ingress", nil)
		}).Should(Equal(2.0))

		noMessage := "76 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - -"
		_, err = fmt.Fprint(conn, noMessage)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("invalid_ingress", nil)
		}).Should(Equal(3.0))

		noPriority := "88 <->1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - - just a test\n"
		_, err = fmt.Fprint(conn, noPriority)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("invalid_ingress", nil)
		}).Should(Equal(4.0))
	})

	It("closes syslog connection when invalid envelope is sent", func() {
		server, spyMetrics, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		counterMessage := buildSyslog(fmt.Sprintf(counterDataFormat, "some-counter", "99", "d"))
		invalidMessage := buildSyslog(counterMessage)
		_, err = fmt.Fprint(conn, invalidMessage)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("invalid_ingress", nil)
		}).Should(Equal(1.0))

		buf := make([]byte, 1024)
		_, err = conn.Read(buf)
		Expect(err).To(Equal(io.EOF))
	})

	DescribeTable("increments invalid ingress on invalid envelope data", func(rfc5424Log string) {
		server, spyMetrics, _ := newTlsServerTestSetup()
		defer server.Stop()
		tlsConfig := buildClientTLSConfig(tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384)
		conn, err := tlsClientConnection(server.Addr(), tlsConfig)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(conn, rfc5424Log)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			return spyMetrics.HasMetric("invalid_ingress", nil)
		}).Should(BeTrue())

		Eventually(func() float64 {
			return spyMetrics.GetMetric("invalid_ingress", nil).Value()
		}).Should(Equal(1.0))
	},
		Entry("Counter - invalid delta", buildSyslog(fmt.Sprintf(counterDataFormat, "some-counter", "99", "d"))),
		Entry("Counter - invalid total", buildSyslog(fmt.Sprintf(counterDataFormat, "some-counter", "dd", "9"))),
		Entry("Gauge - no unit provided", buildSyslog(fmt.Sprintf(gaugeDataFormat, "cpu", "0.23", "blah=\"percentage\""))),
		Entry("Gauge - invalid value", buildSyslog(fmt.Sprintf(gaugeDataFormat, "cpu", "dddd", "unit=\"percentage\""))),
	)

	Describe("TLS security", func() {
		DescribeTable("allows only supported TLS versions", func(clientTLSVersion int, serverShouldAllow bool) {
			server, _, _ := newTlsServerTestSetup()
			defer server.Stop()
			tlsConfig := buildClientTLSConfig(uint16(clientTLSVersion), tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
			_, err := tlsClientConnection(server.Addr(), tlsConfig)

			if serverShouldAllow {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
			Entry("unsupported SSL 3.0", tls.VersionSSL30, false),
			Entry("unsupported TLS 1.0", tls.VersionTLS10, false),
			Entry("unsupported TLS 1.1", tls.VersionTLS11, false),
			Entry("supported TLS 1.2", tls.VersionTLS12, true),
		)

		DescribeTable("allows only supported TLS versions", func(cipherSuite uint16, serverShouldAllow bool) {
			server, _, _ := newTlsServerTestSetup()
			defer server.Stop()
			tlsConfig := buildClientTLSConfig(tls.VersionTLS12, cipherSuite)
			_, err := tlsClientConnection(server.Addr(), tlsConfig)

			if serverShouldAllow {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
			Entry("unsupported cipher RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_RC4_128_SHA", tls.TLS_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA256", tls.TLS_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA", tls.TLS_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_GCM_SHA256", tls.TLS_RSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher RSA_WITH_AES_256_CBC_SHA", tls.TLS_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_256_GCM_SHA384", tls.TLS_RSA_WITH_AES_256_GCM_SHA384, false),

			Entry("supported cipher ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true),
			Entry("supported cipher ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, true),
		)
	})
})

func buildSyslog(structuredData string) string {
	msg := fmt.Sprintf("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [%s] \n", structuredData)
	return fmt.Sprintf("%d %s", len(msg), msg)
}

func buildSyslogWithTags(structuredData, tags string) string {
	msg := fmt.Sprintf("<14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [%s][tags@47450 %s] \n", structuredData, tags)
	return fmt.Sprintf("%d %s", len(msg), msg)
}

func waitForServerToStart(server *syslog.Server) {
	Eventually(server.Addr, "1s", "100ms").ShouldNot(BeEmpty())
}

const counterDataFormat = `counter@47450 name="%s" total="%s" delta="%s"`
const gaugeDataFormat = `gauge@47450 name="%s" value="%s" %s`

func buildClientTLSConfig(maxVersion, cipherSuite uint16) *tls.Config {
	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithIdentityFromFile(
			testing.LogCacheTestCerts.Cert("log-cache"),
			testing.LogCacheTestCerts.Key("log-cache"),
		),
	).Client()
	Expect(err).ToNot(HaveOccurred())

	tlsConf.MaxVersion = uint16(maxVersion)
	tlsConf.CipherSuites = []uint16{cipherSuite}
	tlsConf.InsecureSkipVerify = true

	return tlsConf
}

func tlsClientConnection(addr string, tlsConf *tls.Config) (*tls.Conn, error) {
	dialer := &net.Dialer{}
	dialer.Timeout = time.Second
	return tls.DialWithDialer(dialer, "tcp", addr, tlsConf)
}

func tcpClientConnection(addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}
