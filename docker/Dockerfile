FROM golang:1.8 
MAINTAINER mheese

RUN apt-get update && \
    apt-get install -y pkg-config libsystemd-dev git gcc curl && \
    apt-get purge -y --auto-remove && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /go/src/github.com/mheese/journalbeat && mkdir -p /data
COPY . /go/src/github.com/mheese/journalbeat
WORKDIR /go/src/github.com/mheese/journalbeat
RUN go build && cp journalbeat / && cp journalbeat.yml / && rm -rf /go/src/github.com/mheese
WORKDIR /

CMD ["./journalbeat", "-e", "-c", "journalbeat.yml"]
