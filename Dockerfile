FROM centos:7
MAINTAINER asteris-llc

# General golang dependencies
RUN yum install -y git tar && yum -y clean all
# Specific journalbeat build dependencies
RUN yum install -y gcc systemd systemd-devel && yum -y clean all

# The version of golang in centos repos is too old to compile journalbeat, so we
# use gimme to get a more recent version.
RUN mkdir -p /go/src/github.com/mheese/journalbeat
ENV GOPATH /go
ENV GOBIN /go/bin
ENV GIMME_ENV_PREFIX /go/envs
ENV GIMME_VERSION_PREFIX /go/versions
RUN curl -sLo /go/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme \
  && chmod +x /go/gimme \
  && /go/gimme 1.6.2

WORKDIR /go/src/github.com/mheese/journalbeat
CMD source $GIMME_ENV_PREFIX/go1.6.2.env \
  && go get . \
  && go build -o /tmp/journalbeat github.com/mheese/journalbeat \
  && tar -C /tmp -cvf /tmp/journalbeat.tar.gz journalbeat \
  && chmod 0666 /tmp/journalbeat.tar.gz \
