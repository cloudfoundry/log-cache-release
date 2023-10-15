# Log Cache Release

The in-memory caching layer for logs and metrics in [Cloud Foundry]. Log Cache is a collection of microservices packaged and distributed as a [BOSH] release.

## Getting started

Some fundamental knowledge of [BOSH], [Cloud Foundry], and [Golang](https://go.dev/) are recommended in order to grok this repo and its contents.

### Prerequisites

* [BOSH]: a deployment on an IAAS that supports [Ubuntu Stemcells].

### Deployment

See the `log-cache` instance group within [cf-deployment]. It is not recommended to run this release outside of a Cloud Foundry deployment.

#### Jobs

This release contains the following jobs:

* **log-cache ([spec](jobs/log-cache/spec)) ([main.go](src/cmd/log-cache/main.go))**: an in-memory cache for [loggregator envelopes].
* **log-cache-cf-auth-proxy ([spec](jobs/log-cache-cf-auth-proxy/spec)) ([main.go](src/cmd/cf-auth-proxy/main.go))**: authenticates Log Cache client HTTPS requests with [UAA](https://github.com/cloudfoundry/uaa) or [Cloud Controller] and proxies them to the log-cache-gateway.
* **log-cache-gateway ([spec](jobs/log-cache-gateway/spec)) ([main.go](src/cmd/gateway/main.go))**: accepts HTTP requests for logs and metrics from Log Cache clients and forwards them to log-cache via gRPC.
* **log-cache-syslog-server ([spec](jobs/log-cache-syslog-server/spec)) ([main.go](src/cmd/syslog-server/main.go))**: accepts incoming [loggregator envelopes] via [syslog](https://en.wikipedia.org/wiki/Syslog) and forwards them to log-cache via gRPC.

### Learn more

* [Logging and metrics in Cloud Foundry](https://docs.cloudfoundry.org/loggregator/data-sources.html)
* [Advanced documentation](docs)
* [#logging-and-metrics](https://cloudfoundry.slack.com/archives/CUW93AF3M) in Cloud Foundry Slack

## FAQ

### Accessing the Log Cache directly

To access logs and metrics in Log Cache directly, install the [Log Cache cf CLI plugin](https://github.com/cloudfoundry/log-cache-cli#installing) or query the [API](src/README.md). Only authenticated clients can communicate with Log Cache, and the logs and metrics that can be retrieved are determined by the level of access your authentication allows.

> [!NOTE]
> Accessing Log Cache directly is the only way to retrieve system component metrics from Log Cache.

### System component logs

System component logs are not stored within Log Cache at present. This is in order to prioritize application logs and metrics, as well as system component metrics.

### Reliability during deployments

Log Cache is an in-memory database, and as such will drop its entire cache when it is restarted. Because of that, clients should plan for Log Cache to not be available 100% of the time. For example, [Cloud Controller] depends on Log Cache, but can can function without it, and informs its users that Log Cache is unavailable when necessary.

### Availability during AZ failures

Log Cache is built to be horizontally scalable by hashing source IDs (e.g. application GUID, unique string, etc) of logs and metrics, and assigning them to Log Cache nodes. This assignment is done during deployments, so logs and metrics assigned to a Log Cache node that becomes unavailable due to an AZ failure will also be unavailable.

This situation can be rectified in case of an AZ failure by redeploying with a configuration that does not attempt to place Log Cache in the AZ that is experiencing a failure.


[BOSH]: https://bosh.io/docs/
[cf-deployment]: https://github.com/cloudfoundry/cf-deployment
[Cloud Controller]: https://github.com/cloudfoundry/cloud_controller_ng
[Cloud Foundry]: https://www.cloudfoundry.org/
[loggregator envelopes]: https://github.com/cloudfoundry/loggregator-api#v2-envelope
