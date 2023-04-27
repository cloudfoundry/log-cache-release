package cache_test

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"time"

	"code.cloudfoundry.org/go-metric-registry/testhelpers"
	"code.cloudfoundry.org/tlsconfig"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	rpc "code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	. "code.cloudfoundry.org/log-cache/internal/cache"

	"code.cloudfoundry.org/log-cache/internal/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func tlsLogCacheTestSetup() (*LogCache, *testing.SpyLogCache, *testhelpers.SpyMetricsRegistry, *tls.Config) {
	clientTlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(testing.LogCacheTestCerts.Cert("log-cache"), testing.LogCacheTestCerts.Key("log-cache")),
	).Client(
		tlsconfig.WithAuthorityFromFile(testing.LogCacheTestCerts.CA()),
		tlsconfig.WithServerName("log-cache"),
	)
	Expect(err).ToNot(HaveOccurred())
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(testing.LogCacheTestCerts.Cert("log-cache"), testing.LogCacheTestCerts.Key("log-cache")),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(testing.LogCacheTestCerts.CA()),
	)
	Expect(err).ToNot(HaveOccurred())

	peer := testing.NewSpyLogCache(tlsConfig)
	peerAddr := peer.Start()
	spyMetrics := testhelpers.NewMetricsRegistry()

	cache := New(
		spyMetrics,
		log.New(io.Discard, "", 0),
		WithAddr("127.0.0.1:0"),
		WithClustered(0, []string{"my-addr", peerAddr},
			grpc.WithTransportCredentials(credentials.NewTLS(clientTlsConfig)),
		),
		WithServerOpts(
			grpc.Creds(credentials.NewTLS(tlsConfig)),
		),
	)
	cache.Start()
	return cache, peer, spyMetrics, clientTlsConfig
}

func logCacheTestSetup() (*LogCache, *testing.SpyLogCache, *testhelpers.SpyMetricsRegistry) {
	var err error
	Expect(err).ToNot(HaveOccurred())

	peer := testing.NewSpyLogCache(nil)
	peerAddr := peer.Start()
	spyMetrics := testhelpers.NewMetricsRegistry()

	cache := New(
		spyMetrics,
		log.New(io.Discard, "", 0),
		WithAddr("127.0.0.1:0"),
		WithClustered(0, []string{"my-addr", peerAddr},
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	cache.Start()

	return cache, peer, spyMetrics
}

var _ = Describe("LogCache", func() {
	Describe("TLS security", func() {
		DescribeTable("allows only supported TLS versions", func(clientTLSVersion int, serverAllows bool) {
			cache, _, _, tlsConfig := tlsLogCacheTestSetup()
			defer cache.Close()
			clientTlsConfig := tlsConfig.Clone()
			clientTlsConfig.MaxVersion = uint16(clientTLSVersion)
			clientTlsConfig.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}

			insecureConn, err := grpc.Dial(
				cache.Addr(),
				grpc.WithTransportCredentials(
					credentials.NewTLS(clientTlsConfig),
				),
			)
			Expect(err).NotTo(HaveOccurred())

			insecureClient := rpc.NewEgressClient(insecureConn)
			_, err = insecureClient.Meta(context.Background(), &rpc.MetaRequest{})

			if serverAllows {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

			Entry("unsupported SSL 3.0", tls.VersionSSL30, false), //nolint:staticcheck
			Entry("unsupported TLS 1.0", tls.VersionTLS10, false),
			Entry("unsupported TLS 1.1", tls.VersionTLS11, false),
			Entry("supported TLS 1.2", tls.VersionTLS12, true),
		)

		DescribeTable("allows only supported cipher suites", func(clientCipherSuite uint16, serverAllows bool) {
			cache, _, _, tlsConfig := tlsLogCacheTestSetup()
			defer cache.Close()
			clientTlsConfig := tlsConfig.Clone()
			clientTlsConfig.MaxVersion = tls.VersionTLS12
			clientTlsConfig.CipherSuites = []uint16{clientCipherSuite}

			insecureConn, err := grpc.Dial(
				cache.Addr(),
				grpc.WithTransportCredentials(
					credentials.NewTLS(clientTlsConfig),
				),
			)
			Expect(err).NotTo(HaveOccurred())

			insecureClient := rpc.NewEgressClient(insecureConn)
			_, err = insecureClient.Meta(context.Background(), &rpc.MetaRequest{})

			if serverAllows {
				Expect(err).NotTo(HaveOccurred())
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

	It("returns tail of data filtered by source ID", func() {
		cache, _, spyMetrics, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		writeEnvelopes(cache.Addr(), []*loggregator_v2.Envelope{
			// src-zero hashes to 6727955504463301110 (route to node 0)
			{Timestamp: 1, SourceId: "src-zero"},
			// other-src hashes to 2416040688038506749 (route to node 1)
			{Timestamp: 2, SourceId: "other-src"},
			{Timestamp: 3, SourceId: "src-zero"},
			{Timestamp: 4, SourceId: "src-zero"},
		})

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewEgressClient(conn)

		var es []*loggregator_v2.Envelope
		f := func() error {
			resp, err := client.Read(context.Background(), &rpc.ReadRequest{
				SourceId:   "src-zero",
				Descending: true,
				Limit:      2,
			})
			if err != nil {
				return err
			}

			if len(resp.Envelopes.Batch) != 2 {
				return errors.New("expected 2 envelopes")
			}

			es = resp.Envelopes.Batch
			return nil
		}
		Eventually(f).Should(BeNil())

		Expect(es[0].Timestamp).To(Equal(int64(4)))
		Expect(es[0].SourceId).To(Equal("src-zero"))
		Expect(es[1].Timestamp).To(Equal(int64(3)))
		Expect(es[1].SourceId).To(Equal("src-zero"))

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("log_cache_ingress", nil)
		}).Should(Equal(3.0))

		Eventually(func() float64 {
			return spyMetrics.GetMetricValue("log_cache_egress", nil)
		}).Should(Equal(2.0))
	})

	It("queries data via PromQL Instant Queries", func() {
		cache, _, _, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		now := time.Now()
		writeEnvelopes(cache.Addr(), []*loggregator_v2.Envelope{
			// src-zero hashes to 6727955504463301110 (route to node 0)
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				SourceId:  "src-zero",
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "metric",
						Total: 99,
					},
				},
			},
		})

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewPromQLQuerierClient(conn)

		f := func() error {
			resp, err := client.InstantQuery(context.Background(), &rpc.PromQL_InstantQueryRequest{
				Query: `metric{source_id="src-zero"}`,
				Time:  testing.FormatTimeWithDecimalMillis(now),
			})
			if err != nil {
				return err
			}

			if len(resp.GetVector().GetSamples()) != 1 {
				return errors.New("expected 1 samples")
			}

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("queries data via PromQL Range Queries", func() {
		cache, _, _, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		now := time.Now()
		writeEnvelopes(cache.Addr(), []*loggregator_v2.Envelope{
			// src-zero hashes to 6727955504463301110 (route to node 0)
			{
				Timestamp: now.Add(-2 * time.Second).UnixNano(),
				SourceId:  "src-zero",
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "metric",
						Total: 99,
					},
				},
			},
		})

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewPromQLQuerierClient(conn)

		f := func() error {
			resp, err := client.RangeQuery(context.Background(), &rpc.PromQL_RangeQueryRequest{
				Query: `metric{source_id="src-zero"}`,
				Start: testing.FormatTimeWithDecimalMillis(now.Add(-time.Minute)),
				End:   testing.FormatTimeWithDecimalMillis(now),
				Step:  "1m",
			})
			if err != nil {
				return err
			}

			Expect(len(resp.GetMatrix().GetSeries())).To(Equal(1))
			Expect(len(resp.GetMatrix().GetSeries()[0].GetPoints())).To(Equal(1))

			return nil
		}
		Eventually(f).Should(BeNil())
	})

	It("routes envelopes to peers", func() {
		cache, peer, _, _ := tlsLogCacheTestSetup()
		defer cache.Close()
		writeEnvelopes(cache.Addr(), []*loggregator_v2.Envelope{
			// src-zero hashes to 6727955504463301110 (route to node 0)
			{Timestamp: 1, SourceId: "src-zero"},
			// other-src hashes to 2416040688038506749 (route to node 1)
			{Timestamp: 2, SourceId: "other-src"},
			{Timestamp: 3, SourceId: "other-src"},
		})

		Eventually(peer.GetEnvelopes).Should(HaveLen(2))
		Expect(peer.GetEnvelopes()[0].Timestamp).To(Equal(int64(2)))
		Expect(peer.GetEnvelopes()[1].Timestamp).To(Equal(int64(3)))
		Expect(peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	It("routes envelopes to peers without tls", func() {
		cache, peer, _ := logCacheTestSetup()
		defer cache.Close()
		writeEnvelopesNoTLS(cache.Addr(), []*loggregator_v2.Envelope{
			// src-zero hashes to 6727955504463301110 (route to node 0)
			{Timestamp: 1, SourceId: "src-zero"},
			// other-src hashes to 2416040688038506749 (route to node 1)
			{Timestamp: 2, SourceId: "other-src"},
			{Timestamp: 3, SourceId: "other-src"},
		})

		Eventually(peer.GetEnvelopes).Should(HaveLen(2))
		Expect(peer.GetEnvelopes()[0].Timestamp).To(Equal(int64(2)))
		Expect(peer.GetEnvelopes()[1].Timestamp).To(Equal(int64(3)))
		Expect(peer.GetLocalOnlyValues()).ToNot(ContainElement(false))
	})

	It("accepts envelopes from peers", func() {
		cache, _, _, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		// src-zero hashes to 6727955504463301110 (route to node 0)
		writeEnvelopes(cache.Addr(), []*loggregator_v2.Envelope{
			{SourceId: "src-zero", Timestamp: 1},
		})

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewEgressClient(conn)

		var es []*loggregator_v2.Envelope
		f := func() error {
			resp, err := client.Read(context.Background(), &rpc.ReadRequest{
				SourceId: "src-zero",
			})
			if err != nil {
				return err
			}

			if len(resp.Envelopes.Batch) != 1 {
				return errors.New("expected 1 envelopes")
			}

			es = resp.Envelopes.Batch
			return nil
		}
		Eventually(f).Should(BeNil())

		Expect(es[0].Timestamp).To(Equal(int64(1)))
		Expect(es[0].SourceId).To(Equal("src-zero"))
	})

	It("routes query requests to peers", func() {
		cache, peer, _, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		peer.ReadEnvelopes["other-src"] = func() []*loggregator_v2.Envelope {
			return []*loggregator_v2.Envelope{
				{Timestamp: 99},
				{Timestamp: 101},
			}
		}

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		client := rpc.NewEgressClient(conn)

		// other-src hashes to 2416040688038506749 (route to node 1)
		resp, err := client.Read(context.Background(), &rpc.ReadRequest{
			SourceId:      "other-src",
			StartTime:     99,
			EndTime:       101,
			EnvelopeTypes: []rpc.EnvelopeType{rpc.EnvelopeType_LOG},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Envelopes.Batch).To(HaveLen(2))

		Eventually(peer.GetReadRequests).Should(HaveLen(1))
		req := peer.GetReadRequests()[0]
		Expect(req.SourceId).To(Equal("other-src"))
		Expect(req.StartTime).To(Equal(int64(99)))
		Expect(req.EndTime).To(Equal(int64(101)))
		Expect(req.EnvelopeTypes).To(ConsistOf(rpc.EnvelopeType_LOG))
	})

	It("prunes envelopes against a static limit", func() {
		var err error
		Expect(err).ToNot(HaveOccurred())

		peer := testing.NewSpyLogCache(nil)
		peerAddr := peer.Start()
		spyMetrics := testhelpers.NewMetricsRegistry()

		cache := New(
			spyMetrics,
			log.New(io.Discard, "", 0),
			WithAddr("127.0.0.1:0"),
			WithClustered(0, []string{"my-addr", peerAddr},
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			),
			WithMemoryLimit(1),
		)
		cache.Start()
		defer cache.Close()
		peer.MetaResponses = map[string]*rpc.MetaInfo{}

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		ingressClient := rpc.NewIngressClient(conn)
		egressClient := rpc.NewEgressClient(conn)

		sendRequest := &rpc.SendRequest{
			Envelopes: &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
					{SourceId: "src-zero"},
				},
			},
		}

		_, err = ingressClient.Send(context.Background(), sendRequest)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() map[string]*rpc.MetaInfo {
			resp, err := egressClient.Meta(context.Background(), &rpc.MetaRequest{})
			if err != nil {
				return nil
			}

			return resp.Meta
		}, 10).Should(And(
			HaveKeyWithValue("src-zero", &rpc.MetaInfo{
				Count:           1,
				Expired:         7,
				OldestTimestamp: 7,
			}),
		))
	})

	It("returns all meta information", func() {
		cache, peer, _, tlsConfig := tlsLogCacheTestSetup()
		defer cache.Close()
		peer.MetaResponses = map[string]*rpc.MetaInfo{
			"other-src": {
				Count:           1,
				Expired:         2,
				OldestTimestamp: 3,
				NewestTimestamp: 4,
			},
		}

		conn, err := grpc.Dial(cache.Addr(),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		ingressClient := rpc.NewIngressClient(conn)
		egressClient := rpc.NewEgressClient(conn)

		sendRequest := &rpc.SendRequest{
			Envelopes: &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{
					{SourceId: "src-zero"},
				},
			},
		}

		_, err = ingressClient.Send(context.Background(), sendRequest)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() map[string]*rpc.MetaInfo {
			resp, err := egressClient.Meta(context.Background(), &rpc.MetaRequest{})
			if err != nil {
				return nil
			}

			return resp.Meta
		}).Should(And(
			HaveKeyWithValue("src-zero", &rpc.MetaInfo{
				Count: 1,
			}),
			HaveKeyWithValue("other-src", &rpc.MetaInfo{
				Count:           1,
				Expired:         2,
				OldestTimestamp: 3,
				NewestTimestamp: 4,
			}),
		))
	})
})

func writeEnvelopes(addr string, es []*loggregator_v2.Envelope) {
	tlsConfig, err := testing.NewTLSConfig(
		testing.LogCacheTestCerts.CA(),
		testing.LogCacheTestCerts.Cert("log-cache"),
		testing.LogCacheTestCerts.Key("log-cache"),
		"log-cache",
	)
	if err != nil {
		panic(err)
	}
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		panic(err)
	}

	client := rpc.NewIngressClient(conn)
	var envelopes []*loggregator_v2.Envelope
	for _, e := range es {
		envelopes = append(envelopes, &loggregator_v2.Envelope{
			Timestamp: e.Timestamp,
			SourceId:  e.SourceId,
			Message:   e.Message,
		})
	}

	_, err = client.Send(context.Background(), &rpc.SendRequest{
		Envelopes: &loggregator_v2.EnvelopeBatch{
			Batch: envelopes,
		},
	})
	if err != nil {
		panic(err)
	}
}

func writeEnvelopesNoTLS(addr string, es []*loggregator_v2.Envelope) {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}

	client := rpc.NewIngressClient(conn)
	var envelopes []*loggregator_v2.Envelope
	for _, e := range es {
		envelopes = append(envelopes, &loggregator_v2.Envelope{
			Timestamp: e.Timestamp,
			SourceId:  e.SourceId,
			Message:   e.Message,
		})
	}

	_, err = client.Send(context.Background(), &rpc.SendRequest{
		Envelopes: &loggregator_v2.EnvelopeBatch{
			Batch: envelopes,
		},
	})
	if err != nil {
		panic(err)
	}
}
