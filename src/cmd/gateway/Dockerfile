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
RUN cd /log-cache-src && \
    export VERSION=$(cat version) && \
    echo "version:" $VERSION && \
    go build \
    -ldflags "-X main.buildVersion=${VERSION}" \
    -a \
    -installsuffix nocgo \
    -o /srv/log-cache-gateway \
    -mod=vendor \
    ./cmd/gateway

RUN dpkg -l > /builder-dpkg-list

FROM $BASE_IMAGE

COPY --from=builder /builder-dpkg-list /builder-dpkg-list
COPY --from=builder /srv/log-cache-gateway /srv/log-cache-gateway

RUN groupadd --system log-cache-gateway --gid 1000 && \
    useradd --no-log-init --system --gid log-cache-gateway log-cache-gateway --uid 1000

USER 1000:1000
WORKDIR /srv

ENV ADDR=:8081 \
    LOG_CACHE_ADDR=log-cache:8080

EXPOSE 8081
CMD [ "/srv/log-cache-gateway" ]
