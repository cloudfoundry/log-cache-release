package main

import (
	"time"

	"code.cloudfoundry.org/log-cache/internal/config"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/log-cache/internal/tls"
)

// Config is the configuration for a LogCache.
type Config struct {
	Addr string `env:"ADDR, required, report"`

	// QueryTimeout sets the maximum allowed runtime for a single PromQL query.
	// Smaller timeouts are recommended.
	QueryTimeout time.Duration `env:"QUERY_TIMEOUT, report"`

	// MemoryLimitPercent sets the percentage of total system memory to use for the
	// cache. If exceeded, the cache will prune. Default is 50%.
	MemoryLimitPercent uint `env:"MEMORY_LIMIT_PERCENT, report"`

	// MemoryLimit sets the limit of system memory used to a static number of bytes,
	// overriding MemoryLimitPercent. If exceeded, the cache will prune.
	MemoryLimit uint64 `env:"MEMORY_LIMIT_BYTES, report"`

	// MaxPerSource sets the maximum number of items stored per source.
	// Because autoscaler requires a minute of data, apps with more than 1000
	// requests per second will fill up the router logs/metrics in less than a
	// minute. Default is 100000.
	MaxPerSource int `env:"MAX_PER_SOURCE, report"`

	// TruncationInterval sets the delay between invocations of the
	// truncation loop. This is where log-cache checks if memory utilization
	// has gone above MemoryLimitPercent and evicts envelopes if it has.
	// Default is 1s.
	TruncationInterval time.Duration `env:"TRUNCATION_INTERVAL, report"`

	// PrunesPerGC sets the number of consecutive prunes needed to trigger
	// a garbage collectionc call (GC). This setting is to guard against
	// the cache pruning all envelopes before automatic GC updates the
	// metric for memory used by the cache. Lowering the value increases CPU
	// utilization.
	// Default is 3
	PrunesPerGC int64 `env:"PRUNES_PER_GC, report"`

	// NodeIndex determines what data the node stores. It splits up the range
	// of 0 - 18446744073709551615 evenly. If data falls out of range of the
	// given node, it will be routed to theh correct one.
	NodeIndex int `env:"NODE_INDEX, report"`

	// NodeAddrs are all the LogCache addresses (including the current
	// address). They are in order according to their NodeIndex.
	//
	// If NodeAddrs is empty or size 1, then data is not routed as it is
	// assumed that the current node is the only one.
	NodeAddrs []string `env:"NODE_ADDRS, report"`

	TLS           tls.TLS
	MetricsServer config.MetricsServer
	UseRFC339     bool `env:"USE_RFC339"`
}

// LoadConfig creates Config object from environment variables
func LoadConfig() (*Config, error) {
	c := Config{
		Addr:               ":8080",
		QueryTimeout:       10 * time.Second,
		MemoryLimitPercent: 50,
		MaxPerSource:       100000,
		TruncationInterval: 1 * time.Second,
		PrunesPerGC:        int64(3),
		MetricsServer: config.MetricsServer{
			Port: 6060,
		},
	}

	if err := envstruct.Load(&c); err != nil {
		return nil, err
	}

	return &c, nil
}
