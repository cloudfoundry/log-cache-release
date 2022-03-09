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

By default Log Cache receives data from syslog agents which is an add on which runs on all instance groups by default. Log Cache can also be configured
to read from the Reverse Log Proxy instead, though this option is not recommended because of firehose scalability limits.

Log Cache is queried by Cloud Controller for app instance metrics such as CPU usage and memory when retrieving details for applications and 
by the cf cli directly to retrieve recent logs. It can also be queried using the Log Cache CLI plugin to retrieve system component metrics.


## How do I configure it?

### Scaling

Numerous variables affect the retention of Log Cache:

Number of instances - Increasing adds more storage space, allows higher throughput and reduces contention between sources

Max Per Source ID - Increasing allows a higher max storage allowance, but may decrease the storage of less noisy apps on the same node

Memory per instance - Increasing allows more storage in general, but any given instance may not be able to take advantage of that increase due to max per source id

Memory limit - Increasing memory limit allows for more storage, but may cause out of memory errors and crashing if set too high for the total throughput of the system

Larger CPUs - Increasing the CPU budget per instance should allow higher throughput

Log Cache is known to exceed memory limits under high throughput/stress. If you see your log-cache reaching higher memory
then you have set, you might want to scale your log-cache up. Either solely in terms of CPU per instance, or more instances.

You can monitor the performance of log cache per source id (app or platform component) using the Log Cache CLI. The command `cf log-meta` allows viewing
the amount of logs and metrics as well as the period of time for those logs and metrics for each source on the system. This can be used in conjuction with scaling
to target your use cases. For simple pushes, a low retention period may be adequate. For running analysis on metrics for debugging and scaling, higher retention
periods may be desired; although one should remember all logs and metrics will always be lost upon crashes or re-deploys of log-cache.

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
