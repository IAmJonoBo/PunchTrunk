SHELL := /bin/bash
BIN := bin/punchtrunk
VERSION ?= dev
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build run fmt lint hotspots docker sign test

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/punchtrunk

run: build
	$(BIN) --mode fmt,lint,hotspots

fmt:
	trunk fmt

lint:
	trunk check

test:
	go test -v ./...

hotspots: build
	$(BIN) --mode hotspots

docker:
	docker build -t punchtrunk:local .

sign:
	# Example: cosign sign --keyless punchtrunk:local
	@echo "Use cosign to sign your image (keyless OIDC supported)."

