SHELL := /bin/bash
BIN := bin/trunk-orchestrator

.PHONY: build run fmt lint hotspots docker sign

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/trunk-orchestrator

run: build
	$(BIN) --mode fmt,lint,hotspots

fmt:
	trunk fmt

lint:
	trunk check

hotspots: build
	$(BIN) --mode hotspots

docker:
	docker build -t trunk-orchestrator:local .

sign:
	# Example: cosign sign --keyless trunk-orchestrator:local
	@echo "Use cosign to sign your image (keyless OIDC supported)."
