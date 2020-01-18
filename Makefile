export PATH := /usr/local/go/bin:$(PATH)
GOBIN := $(shell go env GOPATH)/bin
PROMU := $(GOBIN)/promu
PREFIX ?= $(shell pwd)
pkgs   = $(shell go list ./... | grep -v /vendor/)

all: format build test

format:
	go fmt $(pkgs)

test:
	go test -v -short $(pkgs)

build: promu
	if [ -f $(GOBIN)/linux_ppc64le/promu ] ; then cp -a $(GOBIN)/linux_ppc64le/promu $(GOBIN)/promu ; fi
	$(PROMU) build --verbose --prefix $(PREFIX)

promu:
	go get -u github.com/prometheus/promu

.PHONY: promu
