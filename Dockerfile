# Copyright (c) 2017 Marcus Heese

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.10 as builder

MAINTAINER mbrooks

RUN apt update &&\
    apt install -y pkg-config libsystemd-dev git gcc curl

COPY . /go/src/github.com/mheese/journalbeat

WORKDIR /go/src/github.com/mheese/journalbeat

RUN go test -race . ./beater &&\
    go build -ldflags '-s -w' -o /journalbeat


FROM debian:stretch-slim

MAINTAINER mbrooks

RUN apt update &&\
    apt install -y ca-certificates &&\
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /journalbeat /

COPY config/journalbeat.yml /

ENTRYPOINT ["/journalbeat"]

CMD ["-e", "-c", "journalbeat.yml"]
