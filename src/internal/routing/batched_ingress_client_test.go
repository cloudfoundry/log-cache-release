package routing_test

import (
	"io/ioutil"
	"log"
	"time"

	"code.cloudfoundry.org/go-metric-registry/testhelpers"

	rpc "code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/v8/rpc/loggregator_v2"
	metrics "code.cloudfoundry.org/go-metric-registry"
	"code.cloudfoundry.org/log-cache/internal/routing"
	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BatchedIngressClient", func() {
	var (
		m             *testhelpers.SpyMetricsRegistry
		spyDropped    metrics.Counter
		ingressClient *spyIngressClient
		c             *routing.BatchedIngressClient
	)

	BeforeEach(func() {
		m = testhelpers.NewMetricsRegistry()
		spyDropped = m.NewCounter("nodeX_dropped", "some help text")
		ingressClient = newSpyIngressClient()
		c = routing.NewBatchedIngressClient(5, time.Hour, ingressClient, spyDropped, m.NewCounter("send_failure", "some help text"), log.New(ioutil.Discard, "", 0))
	})

	It("sends envelopes by batches because of size", func() {
		for i := 0; i < 5; i++ {
			_, err := c.Send(context.Background(), &rpc.SendRequest{
				Envelopes: &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: int64(i)},
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		}

		Eventually(ingressClient.Requests).Should(HaveLen(1))
		Expect(ingressClient.Requests()[0].Envelopes.Batch).To(HaveLen(5))
	})

	It("sends envelopes by batches because of interval", func() {
		c = routing.NewBatchedIngressClient(5, time.Microsecond, ingressClient, spyDropped, m.NewCounter("send_failure", "some help text"), log.New(ioutil.Discard, "", 0))
		_, err := c.Send(context.Background(), &rpc.SendRequest{
			Envelopes: &loggregator_v2.EnvelopeBatch{
				Batch: []*loggregator_v2.Envelope{
					{Timestamp: 1},
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		Eventually(ingressClient.Requests).Should(HaveLen(1))
		Expect(ingressClient.Requests()[0].Envelopes.Batch).To(HaveLen(1))
	})

	It("increments a dropped counter", func() {
		go func(ingressClient *spyIngressClient) {
			for {
				// Force ingress client to block 100ms
				ingressClient.mu.Lock()
				time.Sleep(100 * time.Millisecond)
				ingressClient.mu.Unlock()
			}
		}(ingressClient)

		for i := 0; i < 25000; i++ {
			c.Send(context.Background(), &rpc.SendRequest{
				Envelopes: &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: 1},
					},
				},
			})
		}

		Eventually(func() float64 {
			return m.GetMetricValue("nodeX_dropped", nil)
		}).ShouldNot(BeZero())
	})

	It("sends envelopes with LocalOnly set to true", func() {
		for i := 0; i < 5; i++ {
			_, err := c.Send(context.Background(), &rpc.SendRequest{
				Envelopes: &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: int64(i)},
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		}

		Eventually(ingressClient.Requests).Should(HaveLen(1))
		Expect(ingressClient.Requests()[0].Envelopes.Batch).To(HaveLen(5))
		Expect(ingressClient.Requests()[0].LocalOnly).To(BeTrue())

		for i, e := range ingressClient.Requests()[0].Envelopes.Batch {
			Expect(e).To(Equal(
				&loggregator_v2.Envelope{
					Timestamp: int64(i),
				},
			))
		}
	})

	It("sends envelopes with LocalOnly false with option", func() {
		c = routing.NewBatchedIngressClient(
			5,
			time.Hour,
			ingressClient,
			spyDropped,
			m.NewCounter("send_failure", "fake help text"),
			log.New(ioutil.Discard, "", 0),
			routing.WithLocalOnlyDisabled,
		)

		for i := 0; i < 5; i++ {
			_, err := c.Send(context.Background(), &rpc.SendRequest{
				Envelopes: &loggregator_v2.EnvelopeBatch{
					Batch: []*loggregator_v2.Envelope{
						{Timestamp: int64(i)},
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		}

		Eventually(ingressClient.Requests).Should(HaveLen(1))
		Expect(ingressClient.Requests()[0].LocalOnly).To(BeFalse())
	})
})
