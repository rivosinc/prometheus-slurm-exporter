APP_NAME := prometheus-slurm-exporter
VERSION := 1
GOPATH := $(shell go env GOPATH)
BIN=$(shell pwd)/bin
GO              ?= GO111MODULE=on go

.PHONY: prometheus-slurm-exporter

default: prometheus-slurm-exporter 

clean:
	@echo "Target 'clean' not implemented yet"

run:
	go run main.go

build: prometheus-slurm-exporter

prometheus-slurm-exporter:
	@echo ">> Building executable $(BIN)/$(APP_NAME)"
	@mkdir -p $(BIN)
	@CGO_ENABLED=0 $(GO) build -o $(BIN)/$(APP_NAME)

rpm: prometheus-slurm-exporter
	@tar --transform 's,^,/$(APP_NAME)-$(VERSION)/,'  -cvzf bin/$(APP_NAME)-$(VERSION).tar.gz bin/$(APP_NAME) bin/prometheus-slurm-exporter contrib
	@mkdir -p ~/rpmbuild/BUILD
	@mkdir -p ~/rpmbuild/BUILDROOT
	@mkdir -p ~/rpmbuild/RPMS
	@mkdir -p ~/rpmbuild/SOURCES
	@mkdir -p ~/rpmbuild/SPECS
	@mkdir -p ~/rpmbuild/SRPMS
	@mv bin/$(APP_NAME)-$(VERSION).tar.gz ~/rpmbuild/SOURCES
	rpmbuild --define "version $(VERSION)" -ba rpm/$(APP_NAME).spec

