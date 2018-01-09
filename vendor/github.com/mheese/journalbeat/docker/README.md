# Journalbeat Docker

Journalbeat can be made to run in a Docker container. This documentation goes
over a few of the commands that can be used to manage the Docker containers.

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

## How to use this image

### Start Journalbeat with commandline configuration

Once the docker container has been built, a quick way to get Journalbeat up and
running is to execute the command below:

    $ docker run -e LOGSTASH_HOST=logtashhost:5044 \
      -v /var/tmp/journalbeat:/data \
      -v /var/log/journal:/var/log/journal \
      -v /run/log/journal:/run/log/journal \
      -v /etc/machine-id:/etc/machine-id \
      --name journalbeat mheese/journalbeat

Note: Journalbeat requires access to resources only available on the host machine.
Because of this, Journalbeat only supports host machines running systemd. Make
sure to mount `/var/log/journal`, `/run/log/journal`, and `/etc/machine-id` for
Journalbeat to functional properly.

Although it's not required, mounting the `/data` volume to the host allows for
journal cursor data to be persistent for server reboots, docker image upgrades,
and docker image restarts.

When running with Docker, all application configuration should be set using
environment variables. The following environment variables are setup to be respected:

* LOGSTASH_HOST - The host and beat port for the logstash server. Example: 192.168.1.100:5044

### Start Journalbeat with configuration file

If you need to run Journalbeat with a configuration file, journalbeat.yml, that's
located in your current directory, you can use the Journalbeat image as follows:

    $ docker run -v "$PWD/journalbeat.yml":/journalbeat.yml \
    -v /var/log/journal:/var/log/journal \
    -v /run/log/journal:/run/log/journal \
    -v /etc/machine-id:/etc/machine-id \
    --name journalbeat mheese/journalbeat

### Using a Dockerfile

If you'd like to have a production Journalbeat image with a pre-baked configuration
file, use of a Dockerfile is recommended:

```
FROM mheese/journalbeat

COPY journalbeat.yml ./

CMD ["./journalbeat", "-e", "-c", "journalbeat.yml"]
```

Then, build with `docker build -t my-journalbeat` and deploy with something like
the following:

    $ docker run -d -v /var/log/journal:/var/log/journal \
    -v /run/log/journal:/run/log/journal \
    -v /etc/machine-id:/etc/machine-id \
    my-journalbeat
