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
    -o /srv/log-cache \
    -mod=vendor \
    ./cmd/log-cache

RUN dpkg -l > /builder-dpkg-list

FROM $BASE_IMAGE

COPY --from=builder /builder-dpkg-list /builder-dpkg-list
COPY --from=builder /srv/log-cache /srv/log-cache

RUN groupadd --system log-cache --gid 1000 && \
    useradd --no-log-init --system --gid log-cache log-cache --uid 1000

USER 1000:1000
WORKDIR /srv

ENV ADDR=:8080 \
    CA_PATH="/ca/tls.crt" \
    CERT_PATH="/log-cache-certs/tls.crt" \
    KEY_PATH="/log-cache-certs/tls.key" \
    METRICS_CA_FILE_PATH="/ca/tls.crt" \
    METRICS_CERT_FILE_PATH="/metrics-certs/tls.crt" \
    METRICS_KEY_FILE_PATH="/metrics-certs/tls.key" \
    ENV_DIR="/srv/conf.d"

EXPOSE 8080
CMD [ "/srv/log-cache" ]