package store_test

import (
	"fmt"

	"code.cloudfoundry.org/go-metric-registry/testhelpers"

	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	"code.cloudfoundry.org/log-cache/internal/cache/store"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("store under high concurrent load", func() {
	var (
		wg    sync.WaitGroup
		s     *store.Store
		start time.Time
	)

	BeforeEach(func() {
		sp := newSpyPruner()
		sp.numberToPrune = 128
		sm := testhelpers.NewMetricsRegistry()
		s = store.NewStore(2500, TruncationInterval, PrunesPerGC, sp, sm)

		start = time.Now()
		var envelopesWritten uint64

		// 10 writers per sourceId, 10k envelopes per writer
		for sourceId := 0; sourceId < 10; sourceId++ {
			for writers := 0; writers < 10; writers++ {
				wg.Add(1)
				go func(sourceId string) {
					defer wg.Done()

					for envelopes := 0; envelopes < 500; envelopes++ {
						e := buildTypedEnvelope(time.Now().UnixNano(), sourceId, &loggregator_v2.Log{})
						s.Put(e, sourceId)
						atomic.AddUint64(&envelopesWritten, 1)
						time.Sleep(50 * time.Microsecond)
					}
				}(fmt.Sprintf("index-%d", sourceId))
			}
		}

		for readers := 0; readers < 10; readers++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					s.Meta()
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}
	})

	It("works", func() {
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		for {
			select {
			case <-done:
				return
			default:
				envelopes := s.Get("index-9", start, time.Now(), nil, nil, 100000, false)
				Expect(len(envelopes)).Should(BeNumerically("<=", 2500))
				time.Sleep(time.Duration(time.Millisecond * 10))
			}
		}
	})
})
