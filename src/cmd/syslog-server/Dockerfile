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
    -o /srv/syslog-server \
    -mod=vendor \
    ./cmd/syslog-server

RUN dpkg -l > /builder-dpkg-list

FROM $BASE_IMAGE

COPY --from=builder /builder-dpkg-list /builder-dpkg-list
COPY --from=builder /srv/syslog-server /srv/syslog-server

RUN groupadd --system syslog-server --gid 1000 && \
    useradd --no-log-init --system --gid syslog-server syslog-server --uid 1000

USER 1000:1000
WORKDIR /srv

ENV SYSLOG_PORT=8082 \
    LOG_CACHE_ADDR=log-cache:8080

EXPOSE 8082
CMD [ "/srv/syslog-server" ]
