package syslog_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-metric-registry/testhelpers"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/log-cache/internal/syslog"
	"code.cloudfoundry.org/log-cache/internal/testing"
)

const (
	LOG_MSG     = `145 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - [tags@47450 key="value" source_type="actual-source-type"] just a test` + "\n"
	COUNTER_MSG = `148 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/1] - [counter@47450 name="test" total="10" delta="5"][tags@47450 key="value"] ` + "\n"
	GAUGE_MSG   = `132 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/3] - [gauge@47450 name="cpu" value="0.23" unit="percentage"] ` + "\n"
	EVENT_MSG   = `128 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - [event@47450 title="event-title" body="event-body"] ` + "\n"
	TIMER_MSG   = `128 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - [timer@47450 name="some-name" start="10" stop="20"] ` + "\n"
)

var _ = Describe("Server", func() {
	var (
		spyRegistry *testhelpers.SpyMetricsRegistry

		serverPort int
		serverOpts []syslog.ServerOption
		server     *syslog.Server
	)

	BeforeEach(func() {
		spyRegistry = testhelpers.NewMetricsRegistry()

		serverPort = 8000 + GinkgoParallelProcess()
		serverOpts = []syslog.ServerOption{
			syslog.WithServerPort(serverPort),
			syslog.WithIdleTimeout(100 * time.Millisecond),
		}
	})

	JustBeforeEach(func() {
		l := log.New(GinkgoWriter, "", log.LstdFlags)
		server = syslog.NewServer(l, spyRegistry, serverOpts...)
		go server.Start()
	})

	JustAfterEach(func() {
		server.Stop()
	})

	Context("when configured with mTLS", func() {
		var clientConn *tls.Conn

		BeforeEach(func() {
			serverOpts = append(
				serverOpts,
				syslog.WithServerTLS(testing.LogCacheTestCerts.Cert("log-cache"), testing.LogCacheTestCerts.Key("log-cache")),
				syslog.WithSyslogClientCA(testing.LogCacheTestCerts.CA()),
			)
		})

		JustBeforeEach(func() {
			opt := tlsconfig.WithIdentityFromFile(testing.LogCacheTestCerts.Cert("log-cache"), testing.LogCacheTestCerts.Key("log-cache"))
			cfg, err := tlsconfig.Build(opt).Client()
			Expect(err).NotTo(HaveOccurred())
			cfg.InsecureSkipVerify = true
			Eventually(func() error {
				var err error
				clientConn, err = tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", fmt.Sprintf("127.0.0.1:%d", serverPort), cfg)
				return err
			}, "5s").Should(Succeed())
		})

		JustAfterEach(func() {
			clientConn.Close()
		})

		It("closes connection after idle timeout", func() {
			Eventually(func() error {
				_, err := clientConn.Read(make([]byte, 1024))
				return err
			}).Should(MatchError(io.EOF))
		})

		It("keeps the connection open if writes are occurring", func() {
			Consistently(func() error {
				_, err := fmt.Fprint(clientConn, LOG_MSG)
				return err
			}, 1).Should(Succeed())
		})

		It("counts incoming messages", func() {
			for i := 0; i < 5; i++ {
				_, err := fmt.Fprint(clientConn, LOG_MSG)
				Expect(err).ToNot(HaveOccurred())
			}
			Eventually(func() float64 {
				return spyRegistry.GetMetric("ingress", nil).Value()
			}).Should(Equal(5.0))
		})

		DescribeTable("message streaming",
			func(msg string, expected *loggregator_v2.Envelope) {
				_, err := fmt.Fprint(clientConn, msg)
				Expect(err).ToNot(HaveOccurred())

				Expect(server.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{})()).Should(ContainElement(expected))
			},
			Entry("log", LOG_MSG, &loggregator_v2.Envelope{
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
			}),
			Entry("counter", COUNTER_MSG, &loggregator_v2.Envelope{
				InstanceId: "1",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags: map[string]string{
					"key": "value",
				},
				Message: &loggregator_v2.Envelope_Counter{
					Counter: &loggregator_v2.Counter{
						Name:  "test",
						Delta: 5,
						Total: 10,
					},
				},
			}),
			Entry("gauge", GAUGE_MSG, &loggregator_v2.Envelope{
				InstanceId: "3",
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
			}),
			Entry("event", EVENT_MSG, &loggregator_v2.Envelope{
				InstanceId: "2",
				Timestamp:  12345000,
				SourceId:   "test-app-id",
				Tags:       map[string]string{},
				Message: &loggregator_v2.Envelope_Event{
					Event: &loggregator_v2.Event{
						Title: "event-title",
						Body:  "event-body",
					},
				},
			}),
			Entry("timer", TIMER_MSG, &loggregator_v2.Envelope{
				InstanceId: "2",
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
			}),
		)

		DescribeTableSubtree("whitespace trimming",
			func(opt syslog.ServerOption, expected string) {
				const msg = `174 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - [tags@47450 key="value" source_type="actual-source-type"]     just a test with with whitespace    ` + "\n"

				BeforeEach(func() {
					serverOpts = append(serverOpts, opt)
				})

				It("streams a correctly formatted message", func() {
					_, err := fmt.Fprint(clientConn, msg)
					Expect(err).ToNot(HaveOccurred())

					req := loggregator_v2.EgressBatchRequest{}
					Expect(server.Stream(context.Background(), &req)()).Should(ContainElement(
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
									Payload: []byte(expected),
									Type:    loggregator_v2.Log_OUT,
								},
							},
						},
					))
				})
			},
			Entry("default", func(s *syslog.Server) {}, "just a test with with whitespace"),
			Entry("explicitly disabled", syslog.WithServerTrimMessageWhitespace(false), "    just a test with with whitespace    "),
			Entry("explicitly enabled", syslog.WithServerTrimMessageWhitespace(true), "just a test with with whitespace"),
		)

		Context("when max message length is exceeded", func() {
			BeforeEach(func() {
				serverOpts = append(serverOpts, syslog.WithServerMaxMessageLength(128))
			})

			It("drops the message", func() {
				_, err := fmt.Fprint(clientConn, "158 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] 123456789012345678901234567890 [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() float64 {
					return spyRegistry.GetMetric("invalid_ingress", nil).Value()
				}).Should(Equal(1.0))
			})
		})

		Context("when the stream context is cancelled", func() {
			It("cancels the stream", func() {
				ctx, cancel := context.WithCancel(context.Background())
				exitCh := make(chan struct{})
				go func() {
					server.Stream(ctx, &loggregator_v2.EgressBatchRequest{})()
					exitCh <- struct{}{}
				}()
				Consistently(exitCh).ShouldNot(Receive())
				cancel()
				Eventually(exitCh).Should(Receive())
			})
		})

		DescribeTable("invalid messages are dropped and close the client connection",
			func(msg string) {
				_, err := fmt.Fprint(clientConn, msg)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() float64 {
					return spyRegistry.GetMetric("invalid_ingress", nil).Value()
				}).Should(Equal(1.0))

				buf := make([]byte, 1024)
				_, err = clientConn.Read(buf)
				Expect(err).To(Equal(io.EOF))
			},
			Entry("wrong length", "126 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [1] - [gauge@47450 name=\"cpu\" value=\"0.23\" unit=\"percentage\"] \n"),
			Entry("no app name", "79 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname - [APP/2] - - just a test\n"),
			Entry("no process ID", "83 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id - - - just a test\n"),
			Entry("no message", "76 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - -"),
			Entry("no priority", "88 <->1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/2] - - just a test\n"),
			Entry("invalid counter delta", `149 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/1] - [counter@47450 name="test" total="10" delta="d"][tags@47450 key="value"] `+"\n"),
			Entry("invalid counter total", `149 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/1] - [counter@47450 name="test" total="dd" delta="5"][tags@47450 key="value"] `+"\n"),
			Entry("no gauge unit", `114 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/3] - [gauge@47450 name="cpu" value="0.23"] `+"\n"),
			Entry("invalid gauge value", `131 <14>1 1970-01-01T00:00:00.012345+00:00 test-hostname test-app-id [APP/3] - [gauge@47450 name="cpu" value="ddd" unit="percentage"] `+"\n"),
		)
	})

	Context("when not configured with mTLS", func() {
		var clientConn net.Conn

		JustBeforeEach(func() {
			Eventually(func() error {
				var err error
				clientConn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", serverPort))
				return err
			}, "5s").Should(Succeed())
		})

		JustAfterEach(func() {
			clientConn.Close()
		})

		It("accepts tcp connections", func() {
			_, err := fmt.Fprint(clientConn, LOG_MSG)
			Expect(err).ToNot(HaveOccurred())

			Expect(server.Stream(context.Background(), &loggregator_v2.EgressBatchRequest{})()).Should(ContainElement(
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
	})
})
