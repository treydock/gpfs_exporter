# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 armv7 arm64 ppc64le s390x
DOCKER_REPO	 ?= treydock
export GOPATH ?= $(firstword $(subst :, ,$(shell go env GOPATH)))

include Makefile.common

DOCKER_IMAGE_NAME ?= gpfs_exporter

# Avoid 'operation not permitted' errors when tar tries to set promu tar directory permissions
$(PROMU):
	$(eval PROMU_TMP := $(shell mktemp -d))
	curl -s -L $(PROMU_URL) | tar -xvzf - -C $(PROMU_TMP) --strip-components=1
	mkdir -p $(FIRST_GOPATH)/bin
	install -m 0755 $(PROMU_TMP)/promu $(FIRST_GOPATH)/bin/promu
	rm -r $(PROMU_TMP)

coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
