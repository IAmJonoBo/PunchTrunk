SHELL := /bin/bash
BIN := bin/punchtrunk

.PHONY: build run fmt lint hotspots docker sign

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/punchtrunk

run: build
	$(BIN) --mode fmt,lint,hotspots

fmt:
	trunk fmt

lint:
	trunk check

hotspots: build
	$(BIN) --mode hotspots

docker:
	docker build -t punchtrunk:local .

sign:
	# Example: cosign sign --keyless punchtrunk:local
	@echo "Use cosign to sign your image (keyless OIDC supported)."
