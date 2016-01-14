# Journalbeat

Journalbeat is the [Beat](https://www.elastic.co/products/beats) used for log
shipping from systemd/journald based Linux systems. It follows the system journal
very much like `journalctl -f` and sends the data to Logstash/Elasticsearch (or
whatever you configured for your beat).

Journalbeat is targeting pure systemd distributions like CoreOS, Atomic Host, or
others. There are no intentions to add support for older systems that do not use
journald.

## Documentation

None so far. As of this writing, this is the first commit. There are things to
come. You can find a `journalbeat.yml` config file in the `etc` folder which
should be self-explanatory for the time being.

## Build

Without checking out this repository, you can get a compiled version of this by
doing some like the following:

```
mkdir journalbeat
go get github.com/elastic/beats
go get github.com/garyburd/redigo
go get github.com/nranchev/go-libGeoIP
go get github.com/satori/go.uuid
go get gopkg.in/yaml.v2
go get github.com/mheese/go-systemd
go get github.com/mheese/journalbeat
go install github.com/mheese/journalbeat
```

**NOTE:** This is not the preferred way from Elastic on how to do it. Needs to
be revised (of course).

## Caveats

A few current caveats with journalbeat

### go-systemd

Journalbeat currently uses a forked version of [go-systemd](https://github.com/coreos/go-systemd). All changes should be merged back upstream to the repo of CoreOS, and I will work on pull requests soon.

### cgo

The underlying system library [go-systemd](https://github.com/coreos/go-systemd) makes heavy usage of cgo and the final binary will be linked against all client libraries that are needed in order to interact with sd-journal. That means that
the resulting binary is not really platform / distribution independent (which is kind of expected in a way).
