# Journalbeat

Asteris maintains this repo in order to build binary releases for distribution
on CentOS. The only difference between this repo and the upstream is the
Dockerfile (for a consistent build environment), the binary releases, and the CI
build.

To use the Dockerfile, simply run
```bash
docker build -t journalbeat-packaging .
docker run -it -v $PWD:/go/src/github.com/mheese/journalbeat:ro -v $PWD:/tmp:rw journalbeat-packaging
```
