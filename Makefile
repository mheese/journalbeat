# Journalbeats Makefile
#
# This Makefile contains a collection of targets to help with docker image
# maintenance and creation. Run `make docker-build` to build the docker
# image. Run `make docker-tag` to build the image and tag the docker image
# with the current git tag.
#
# Note: This Makefile can be modified to include any future non-docker build
# tasks as well.

IMAGE_NAME := journalbeat
GIT_BRANCH_NAME := $(shell git rev-parse --abbrev-ref HEAD | sed "sX/X-Xg")
GIT_TAG_NAME := $(shell git describe --tags)

TAGS := $(GIT_BRANCH_NAME) $(GIT_TAG_NAME)

ifeq ($(GIT_BRANCH_NAME),master)
  TAGS += latest
endif

TAGS := $(foreach t,$(TAGS),$(IMAGE_NAME):$(t))

#
# Clean up the project
#
clean:
	rm -f Dockerfile
	rm -rf build
.PHONY: clean

#
# Copy the Dockerfile for the build to the main project directory
#
Dockerfile:
	cp docker/dockerfile.build Dockerfile

#
# Make the build directory
#
build: Dockerfile build/journalbeat

#
# Build the journalbeat go image using docker
#
build/journalbeat:
	mkdir -p build
	docker build -t $(IMAGE_NAME)-build .
	docker run --name $(IMAGE_NAME)-build $(IMAGE_NAME)-build
	-docker cp $(IMAGE_NAME)-build:/go/src/github.com/mheese/journalbeat/journalbeat build/journalbeat
	docker rm $(IMAGE_NAME)-build
	docker rmi $(IMAGE_NAME)-build

#
# Copy the Dockerfile for release to the build directory
#
build/Dockerfile:
	cp docker/dockerfile.release build/Dockerfile

#
# Copy the default journalbeat.yml for release to the build directory
#
build/journalbeat.yml:
	cp docker/journalbeat.yml build/journalbeat.yml

#
# docker tag the image
#
docker-tag: docker-build
	@echo $(TAGS) | xargs -n 1 docker tag $(IMAGE_NAME)
.PHONY: docker-tag

#
# docker build the image
#
docker-build: build build/Dockerfile build/journalbeat.yml
	cd build && docker build -t $(IMAGE_NAME) .
.PHONY: docker-build

#
#  show the current version and branch name, for quick reference.
#
version:
	@echo Version: $(GIT_TAG_NAME)
	@echo Branch: $(GIT_BRANCH_NAME)
.PHONY: version
