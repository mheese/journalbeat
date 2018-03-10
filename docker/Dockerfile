FROM golang:1.8 as builder
MAINTAINER mbrooks
RUN apt-get update && \
    apt-get install -y pkg-config libsystemd-dev git gcc curl && \
    apt-get purge -y --auto-remove && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /go/src/github.com/mheese/journalbeat && mkdir -p /data
COPY . /go/src/github.com/mheese/journalbeat
WORKDIR /go/src/github.com/mheese/journalbeat
RUN go build


FROM debian:stretch-slim
MAINTAINER mbrooks
RUN apt-get update && apt install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /go/src/github.com/mheese/journalbeat/journalbeat /
COPY config/journalbeat.yml /
CMD ["/journalbeat", "-e", "-c", "journalbeat.yml"]