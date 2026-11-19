APP=cbt-api
VERSION ?= $(shell date +%Y%m%d-%H%M)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo nogit)

.PHONY: tidy build run

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
	-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(shell date -u +%FT%TZ)" \
	-o bin/$(APP) ./cmd/api
	sha256sum bin/$(APP) > bin/$(APP).sha256

run:
	go run ./cmd/api
