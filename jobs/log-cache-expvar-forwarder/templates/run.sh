#!/bin/bash
# Fetch container ID for AWS...
export INSTANCE_CID=$(curl --max-time 0.5 -sf http://169.254.169.254/latest/meta-data/instance-id)
if [[ -z "${INSTANCE_CID}" ]]; then
    # ... or GCP
    export INSTANCE_CID=$(curl --max-time 0.5 -sf -H 'Metadata-Flavor: Google' http://169.254.169.254/computeMetadata/v1/instance/name)
fi
export METRIC_HOST="${INSTANCE_CID:-}"

exec /var/vcap/packages/log-cache-expvar-forwarder/log-cache-expvar-forwarder
