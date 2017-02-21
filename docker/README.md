# Journalbeat Docker

Journalbeat can be made to run in a Docker container. This documentation goes
over a few of the commands that can be used to manage the Docker container.s

## Build Container

From the main project directory, a docker container can be build like so:

    $ make docker-build

## Tag Container

A docker container can also be built for the current git tag. If the project is
ahead of a tag, the git describe will be used instead.  

    $ make docker-tag

## Cleanup

To remove any temporary files created from the build process, clean can be run:

    $ make clean

## Running The Container

Once the docker container has been built, run it like the following example:

    $ docker run -e LOGSTASH_HOST=logtashhost:5044 \
      -v /var/tmp/journalbeat:/data \
      -v /var/log/journal:/var/log/journal \
      -v /run/log/journal:/run/log/journal \
      -v /etc/machine-id:/etc/machine-id \
      --name journalbeat journalbeat

Note: Journalbeat requires access to resources only available on the host machine.
Because of this, Journalbeat only supports host machines running systemd. Make
sure to mount `/var/log/journal`, `/run/log/journal`, and `/etc/machine-id` for
Journalbeat to functional properly.

When running with Docker, all application configuration should be set using
environment variables. The following environment variables are setup to be respected:

* LOGSTASH_HOST - The host and beat port for the logstash server. Example: 192.168.1.100:5044
