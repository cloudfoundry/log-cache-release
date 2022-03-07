Log Cache Release
=================

## What is Log Cache?

Log Cache provides an in memory caching layer for logs and metrics which are emitted by
applications and system components in Cloud Foundry. 

Log Cache is deployed as a group of nodes which communicate between themselves
over GRPC. Source IDs (such as the CF application ID or a unique string for a
system component) are hashed to determine which Log Cache
instance will cache the data. Envelopes from a source ID can be sent to any log cache instance and
are then forwarded to the assigned node. Similarly queries for Log Cache can be
sent to any node and the query will be forwarded to the appropriate node which
is caching the data for the requested source ID.

## How does Log Cache fit into Cloud Foundry?

Log Cache is included by default in 
[Cloud Foundry's cf-deployment](https://github.com/cloudfoundry/cf-deployment).

By default Log Cache receives data from syslog agents which is an add on which runs on all jobs by default. Log Cache can also be configured
to read from the Reverse Log Proxy instead, though this option is not recommended because of firehose scalability limits.

Log Cache is queried by Cloud Controller for app instance metrics such as CPU usage and memory when retrieving details for applications and 
by the cf cli directly to retrieve recent logs. It can also be queried using the Log Cache CLI plugin to retrieve system component metrics.


## How do I configure it?

### Scaling

We recomend aiming for retaining 15 minutes of logs and metrics in log-cache per source. This can be monitored with the `cf log-meta` command
provided by the Log Cache cli plugin or from the log_cache_cache_period metric reported by Log Cache.

Log Cache nodes use CPU to process incoming envelopes and evict old envelopes from the cache. If your Log Cache runs out of
CPU resources but has an adequate retention interval you can move Log Cache to an instance type with more CPU.

Log Cache nodes use memory to cache envelopes. If your Log Cache retention interval is shorter than you would like but you 
have not exhausted your CPU you can move Log Cache to an instance type with more memory.

In either case you can add more Log Cache nodes to increase both CPU and memory at the same time.

### Reliability

Log Cache is an in memory cache and as such will drop envelopes when it restarts. Users should not expect 100% availability of
logs in Log Cache and should plan accordingly. For example, Cloud Controller can function without Log Cache though users are
informed that Log Cache is unavailable.

## How do I use it?

### From the `cf` CLI

Application developers using Cloud Foundry will use Log Cache automatically. Build logs while running
`cf push` are streamed through Log Cache. Application logs when running `cf logs` are retrieved from Log Cache.
Application metrics when running `cf app APP_NAME` are retrieved from Log Cache.

### Using the Log Cache CLI plugin
To query Log Cache directly users or operators can install the [Log Cache CLI plugin](https://github.com/cloudfoundry/log-cache-cli)
by running `cf install-plugin -r CF-Community "log-cache"` which provides additional commands in the CLI for querying logs and metrics
stored in log cache. This is useful for querying system component metrics which are not exposed otherwise. See the CLI plugin README for details. 

### Log Cache API
Documentation about the internals of Log Cache and its API can be found [here](https://github.com/cloudfoundry/log-cache-release/blob/main/src/README.md)
