module code.cloudfoundry.org/log-cache

go 1.23.0

require (
	code.cloudfoundry.org/go-batching v0.0.0-20241223135445-b8f45a03eb70
	code.cloudfoundry.org/go-diodes v0.0.0-20241223074059-7f8c1f03edeb
	code.cloudfoundry.org/go-envstruct v1.7.0
	code.cloudfoundry.org/go-metric-registry v0.0.0-20241216212740-f99815f94a2a
	code.cloudfoundry.org/tlsconfig v0.14.0
	github.com/Benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/benbjohnson/jmphash v0.0.0-20141216154655-2d58f234cd86
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cloudfoundry/gosigar v1.3.83
	github.com/dvsekhvalnov/jose2go v1.8.0
	github.com/emirpasic/gods v1.18.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.25.1
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20210608084020-ac565dc76ba6
	github.com/onsi/ginkgo/v2 v2.22.2
	github.com/onsi/gomega v1.36.2
	github.com/prometheus/client_golang v1.20.5
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.61.0
	github.com/prometheus/prometheus v1.99.0
	golang.org/x/net v0.33.0
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.1
)

require (
	code.cloudfoundry.org/go-log-cache/v3 v3.0.3
	code.cloudfoundry.org/go-loggregator/v10 v10.0.1
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/go-chi/chi/v5 v5.2.0
	github.com/shirou/gopsutil/v4 v4.24.12
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/ebitengine/purego v0.8.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20241210010833-40e02aabc2ad // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/square/certstrap v1.3.0 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.step.sm/crypto v0.56.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.28.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250102185135-69823020774d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250102185135-69823020774d // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/prometheus/prometheus => github.com/prometheus/prometheus v2.13.1+incompatible // pinned
