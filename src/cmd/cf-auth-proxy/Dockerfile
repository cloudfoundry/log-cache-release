ARG BASE_IMAGE=ubuntu:bionic
FROM $BASE_IMAGE as builder

RUN apt update && \
    apt install --no-install-recommends -y ca-certificates wget unzip && \
    apt-get clean

# Install Go
ARG GOLANG_SOURCE=dl.google.com/go
RUN wget https://$GOLANG_SOURCE/go1.12.7.linux-amd64.tar.gz -O go.tar.gz && \
    tar -xf go.tar.gz && \
    mv go /usr/local && \
    rm go.tar.gz
ENV GOROOT=/usr/local/go
ENV GOPATH=$HOME/go
ENV GO111MODULE=on
ENV PATH=$GOROOT/bin:$GOPATH/bin:$PATH

ENV GOOS=linux \
    GOARCH=amd64 \
    CGO_ENABLED=0

COPY / /log-cache-src/

RUN cd /log-cache-src && go build \
    -a \
    -installsuffix nocgo \
    -o /srv/cf-auth-proxy \
    -mod=vendor \
    ./cmd/cf-auth-proxy

RUN dpkg -l > /builder-dpkg-list

FROM $BASE_IMAGE

COPY --from=builder /builder-dpkg-list /builder-dpkg-list
COPY --from=builder /srv/cf-auth-proxy /srv/cf-auth-proxy

RUN groupadd --system cf-auth-proxy --gid 1000 && \
    useradd --no-log-init --system --gid cf-auth-proxy cf-auth-proxy --uid 1000

USER 1000:1000
WORKDIR /srv

ENV LOG_CACHE_GATEWAY_ADDR=log-cache:8081 \
    ADDR=:8083 \
    INTERNAL_IP="127.0.0.1" \
    CAPI_ADDR="" \
    UAA_ADDR=""

EXPOSE 8083
CMD [ "/srv/cf-auth-proxy" ]
