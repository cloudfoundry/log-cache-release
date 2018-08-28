Log Cache Release
=================

Log Cache Release is a [bosh](https://github.com/cloudfoundry/bosh) release
for [Log Cache](https://code.cloudfoundry.org/log-cache). It provides
an [experimental in memory caching layer](https://docs.google.com/document/d/1yhfl0EB_MkHkh4JdRZXGeQMx_BDMCuB-SpPuSrD3VOU/edit#) as a potential future
alternative for `cf logs --recent` and container metrics retrieval.

### Deploying Log Cache

Log Cache can be deployed either as a standalone deployment or within
[Cloud Foundry](https://github.com/cloudfoundry/cf-deployment).

In both cases Log Cache will have to know about Loggregator.

#### Cloud Config

Every bosh deployment requires a [cloud
config](https://bosh.io/docs/cloud-config.html). The Log Cache deployment
manifest assumes the CF-Deployment cloud config has been uploaded.

#### Creating and Uploading Release

The first step in deploying Log Cache is to create a release. Final releases
are preferable, however during the development process dev releases are
useful.

The following commands will create a dev release and upload it to an
environment named `lite`.

```
bosh create-release --force
bosh -e lite upload-release --rebase
```

#### Standalone

Standalone Log Cache only has to know where to find Loggregator. The
Loggregator CA is named `loggregator_ca`. The given variables file should
include the deployed Loggregator's CA.

The following command will deploy a standalone Log Cache against a Loggregator
that is deployed with a `loggregator-vars.yml` variables file.

```
bosh \
    --environment lite \
    --deployment log-cache deploy manifests/log-cache.yml \
    --vars-store vars.yml \
    --vars-file ~/workspace/loggregator-release/vars.yml
```

#### Cloud Foundry

Log Cache deployed within Cloud Foundry reads from the Loggregator system and
registers with the [GoRouter](https://github.com/cloudfoundry/gorouter) at
`log-cache.<system-domain>` (e.g. for bosh-lite `log-cache.bosh-lite.com`).

As of `cf-deployment` version 3.x, Log Cache is included by default in CF.

The following commands will deploy Log Cache in CF.

```
bosh update-runtime-config \
    ~/workspace/bosh-deployment/runtime-configs/dns.yml
bosh update-cloud-config \
    ~/workspace/cf-deployment/iaas-support/bosh-lite/cloud-config.yml
bosh \
    --environment lite \
    --deployment cf \
    deploy ~/workspace/cf-deployment/cf-deployment.yml \
    --ops-file ~/workspace/cf-deployment/operations/bosh-lite.yml \
    --ops-file ~/workspace/cf-deployment/operations/use-compiled-releases.yml \
    -v system_domain=bosh-lite.com
```

##### Log Cache Client
By Default, Log Cache uses the `doppler` client included with `cf-deployment`.

If you would like to use a custom client, it requires the `uaa.resource` authority:
```
<custom_client_id>:
    authorities: uaa.resource
    override: true
    authorized-grant-types: client_credentials
    secret: <custom_client_secret>
```

### Operating Log Cache
Log cache is currently an experimental release with plans for integrations starting
in summer 2018. That said we have defined the following service level objectives for the
release.

#### Reliability SLO
Log cache depends on Loggregator and is expected to offer slightly lower reliability.
This is primarily due to the ephemeral nature of the cache. Loss will occur during a
deployment. Outside of deployments a 99% reliability can be expected.

#### Cache Duration & Scaling
Log cache is horizontally scalable and we recommend scaling based on the formula below.
We have set a service level objective of 15 minutes with this scaling recommendation.
```
Log Cache Nodes = Envelopes Per Second / 10,000
```

Note - this is intentionally designed to match the scaling of the Log Router used in the
Loggregator system for [colocation in cf-deployment][cf-deployment-ops] - that said more
recent testing with this colocation strategy has not met these SLOs. If targeting these
SLOs is critical to your foundation we recommend using a log-cache instance group.


[cf-deployment-ops]:        https://github.com/cloudfoundry/cf-deployment/blob/master/operations/experimental/use-log-cache.yml
