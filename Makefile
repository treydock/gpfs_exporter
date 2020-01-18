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
	$(PROMU) build --verbose --prefix $(PREFIX)

promu:
	go get -u github.com/prometheus/promu

.PHONY: promu
