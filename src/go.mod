module code.cloudfoundry.org/log-cache

go 1.21.0

toolchain go1.21.9

require (
	code.cloudfoundry.org/go-batching v0.0.0-20240325232529-c21ea48767e2
	code.cloudfoundry.org/go-diodes v0.0.0-20240419195010-376885f5f3d4
	code.cloudfoundry.org/go-envstruct v1.7.0
	code.cloudfoundry.org/go-log-cache v1.0.1-0.20221018060643-5866ce9a4570
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1
	code.cloudfoundry.org/go-metric-registry v0.0.0-20240325232813-eb1144b007e4
	code.cloudfoundry.org/tlsconfig v0.0.0-20240423163804-1b0dcf57fddb
	github.com/Benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/benbjohnson/jmphash v0.0.0-20141216154655-2d58f234cd86
	github.com/cespare/xxhash v1.1.0
	github.com/cloudfoundry/gosigar v1.3.55
	github.com/dvsekhvalnov/jose2go v1.7.0
	github.com/emirpasic/gods v1.18.1
	github.com/gorilla/mux v1.8.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20210608084020-ac565dc76ba6
	github.com/onsi/ginkgo/v2 v2.17.1
	github.com/onsi/gomega v1.33.0
	github.com/prometheus/client_golang v1.19.0
	github.com/prometheus/client_model v0.6.1
	github.com/prometheus/common v0.53.0
	github.com/prometheus/prometheus v2.13.1+incompatible // pinned
	github.com/shirou/gopsutil/v3 v3.24.3
	golang.org/x/net v0.24.0
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.33.0
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/alecthomas/units v0.0.0-20231202071711-9a357b53e9c9 // indirect
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-kit/kit v0.13.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20240416155748-26353dc0451f // indirect
	github.com/lufia/plan9stats v0.0.0-20240408141607-282e7b5d6b74 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/procfs v0.14.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/square/certstrap v1.3.0 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.step.sm/crypto v0.44.7 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.20.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240415180920-8c6c420018be // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240415180920-8c6c420018be // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/prometheus/prometheus => github.com/prometheus/prometheus v2.13.1+incompatible // pinned
