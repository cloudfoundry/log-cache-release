#!/usr/bin/env bash

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

docker build -t logcache/log-cache:latest ${SCRIPT_ROOT}/src -f ${SCRIPT_ROOT}/src/cmd/log-cache/Dockerfile
docker build -t logcache/syslog-server:latest ${SCRIPT_ROOT}/src -f ${SCRIPT_ROOT}/src/cmd/syslog-server/Dockerfile
docker build -t logcache/log-cache-gateway:latest ${SCRIPT_ROOT}/src -f ${SCRIPT_ROOT}/src/cmd/gateway/Dockerfile
docker build -t logcache/log-cache-cf-auth-proxy:latest ${SCRIPT_ROOT}/src -f ${SCRIPT_ROOT}/src/cmd/cf-auth-proxy/Dockerfile