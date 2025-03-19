# Scaling

A factsheet for operators to consider when scaling Log Cache.

---

## Variables to tweak

Numerous variables affect the retention of Log Cache:

* **Number of nodes**: Increasing the number of Log Cache nodes adds more storage space, allows higher throughput and reduces contention between sources.
* **Max envelopes per source ID**: Increasing this allows more envelopes to be stored per source ID a higher max storage allowance, but may decrease the storage of less noisy apps on the same node.
* **Memory per instance**: Increasing allows more storage in general, but any given instance may not be able to take advantage of that increase due to max per source id

Memory limit - Increasing memory limit allows for more storage, but may cause out of memory errors and crashing if set too high for the total throughput of the system

Larger CPUs - Increasing the CPU budget per instance should allow higher throughput

Log Cache is known to exceed memory limits under high throughput/stress. If you see your log-cache reaching higher memory
then you have set, you might want to scale your log-cache up. Either solely in terms of CPU per instance, or more instances.

You can monitor the performance of log cache per source id (app or platform component) using the Log Cache CLI. The command `cf log-meta` allows viewing
the amount of logs and metrics as well as the period of time for those logs and metrics for each source on the system. This can be used in conjunction with scaling
to target your use cases. For simple pushes, a low retention period may be adequate. For running analysis on metrics for debugging and scaling, higher retention
periods may be desired; although one should remember all logs and metrics will always be lost upon crashes or re-deploys of log-cache.
