module code.cloudfoundry.org/log-cache

go 1.26.0

require (
	code.cloudfoundry.org/go-batching v0.0.0-20260831144019-2a4de114ff3e
	code.cloudfoundry.org/go-diodes v0.0.0-20260831145205-e8366a756183
	code.cloudfoundry.org/go-envstruct v1.7.0
	code.cloudfoundry.org/go-metric-registry v0.0.0-20260901100552-589270c3b9ea
	code.cloudfoundry.org/tlsconfig v0.65.0
	github.com/Benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/benbjohnson/jmphash v0.0.0-20141216154655-2d58f234cd86
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cloudfoundry/gosigar v1.3.126
	github.com/dvsekhvalnov/jose2go v1.10.0
	github.com/emirpasic/gods v1.18.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0
	github.com/onsi/ginkgo/v2 v2.32.1
	github.com/onsi/gomega v1.43.0
	github.com/prometheus/client_golang v1.24.1
	github.com/prometheus/client_model v0.6.3
	github.com/prometheus/common v0.71.0
	github.com/prometheus/prometheus v1.99.0
	golang.org/x/net v0.58.0 // indirect
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	code.cloudfoundry.org/go-log-cache/v3 v3.1.2
	code.cloudfoundry.org/go-loggregator/v10 v10.3.1
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/go-chi/chi/v5 v5.3.2
	github.com/leodido/go-syslog/v4 v4.6.0
	github.com/shirou/gopsutil/v4 v4.26.8
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260903180319-d6c3cb2f37ec // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lufia/plan9stats v0.0.0-20260802145828-341c2f0c90b5 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/square/certstrap v1.3.0 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.step.sm/crypto v0.90.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/mod v0.40.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260904194346-d0f1323225a4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260904194346-d0f1323225a4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/prometheus/prometheus => github.com/prometheus/prometheus v2.13.1+incompatible // pinned
