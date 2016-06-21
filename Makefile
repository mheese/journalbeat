TEST?=. ./beat
NAME = $(shell awk -F\" '/^const Name/ { print $$2 }' main.go)
VERSION = $(shell awk -F\" '/^const Version/ { print $$2 }' main.go)
DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PWD = $(shell pwd)

all: build

deps:
	go get -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

updatedeps:
	go get -u -v ./...
	echo $(DEPS) | xargs -n1 go get -d

build: deps
	@mkdir -p bin/
	go build -o bin/$(NAME)

test: deps
	go test $(TEST) $(TESTARGS)
	go vet $(TEST)

centos:
	@mkdir -p build/
	cat ci/centos.dockerfile.part ci/common.dockerfile.part > Dockerfile
	docker build -t centos-$(NAME)-packaging .
	docker run -it -v $(PWD):/go/src/github.com/mheese/$(NAME):rw centos-$(NAME)-packaging
	rm Dockerfile

fedora:
	@mkdir -p build/
	cat ci/fedora.dockerfile.part ci/common.dockerfile.part > Dockerfile
	docker build -t fedora-$(NAME)-packaging .
	docker run -it -v $(PWD):/go/src/github.com/mheese/$(NAME):rw fedora-$(NAME)-packaging
	rm Dockerfile

package: centos fedora

.PHONY: all deps updatedeps build test package
