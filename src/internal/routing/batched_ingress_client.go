package routing

import (
	"log"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"

	batching "code.cloudfoundry.org/go-batching"
	diodes "code.cloudfoundry.org/go-diodes"
	rpc "code.cloudfoundry.org/go-log-cache/v3/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// BatchedIngressClient batches envelopes before sending it. Each invocation
// to Send is async.
type BatchedIngressClient struct {
	c rpc.IngressClient

	buffer            *diodes.OneToOne
	size              int
	interval          time.Duration
	log               *log.Logger
	sendFailureMetric metrics.Counter

	localOnly bool
}

type BatchedIngressClientOption func(b *BatchedIngressClient)

func WithLocalOnlyDisabled(b *BatchedIngressClient) {
	b.localOnly = false
}

// NewBatchedIngressClient returns a new BatchedIngressClient.
func NewBatchedIngressClient(
	size int,
	interval time.Duration,
	c rpc.IngressClient,
	droppedMetric metrics.Counter,
	sendFailureMetric metrics.Counter,
	log *log.Logger,
	opts ...BatchedIngressClientOption,
) *BatchedIngressClient {
	b := &BatchedIngressClient{
		c:                 c,
		size:              size,
		interval:          interval,
		log:               log,
		sendFailureMetric: sendFailureMetric,
		localOnly:         true,

		buffer: diodes.NewOneToOne(10000, diodes.AlertFunc(func(dropped int) {
			log.Printf("dropped %d envelopes", dropped)
			droppedMetric.Add(float64(dropped))
		})),
	}

	for _, opt := range opts {
		opt(b)
	}

	go b.start()

	return b
}

// Send batches envelopes before shipping them to the client.
func (b *BatchedIngressClient) Send(ctx context.Context, in *rpc.SendRequest, opts ...grpc.CallOption) (*rpc.SendResponse, error) {
	for i := range in.GetEnvelopes().GetBatch() {
		b.buffer.Set(diodes.GenericDataType(in.Envelopes.Batch[i]))
	}

	return &rpc.SendResponse{}, nil
}

func (b *BatchedIngressClient) start() {
	batcher := batching.NewBatcher(b.size, b.interval, batching.WriterFunc(b.write))
	for {
		e, ok := b.buffer.TryNext()
		if !ok {
			batcher.Flush()
			time.Sleep(50 * time.Millisecond)
			continue
		}
		batcher.Write((*loggregator_v2.Envelope)(e))
	}
}

func (b *BatchedIngressClient) write(batch []interface{}) {
	var e []*loggregator_v2.Envelope
	for _, i := range batch {
		e = append(e, i.(*loggregator_v2.Envelope))
	}

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	_, err := b.c.Send(ctx, &rpc.SendRequest{
		LocalOnly: b.localOnly,
		Envelopes: &loggregator_v2.EnvelopeBatch{Batch: e},
	})

	if err != nil {
		b.log.Printf("failed to write %d envelopes: %s", len(e), err)
		b.sendFailureMetric.Add(1)
	}
}
