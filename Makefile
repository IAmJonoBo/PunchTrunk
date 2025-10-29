SHELL := /bin/bash
BIN := bin/punchtrunk
VERSION ?= dev
LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build run fmt lint hotspots docker sign test offline-bundle

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
	@if command -v bats >/dev/null 2>&1; then \
		bats scripts/tests; \
	else \
		echo "bats not found; skipping shell tests" >&2; \
	fi

hotspots: build
	$(BIN) --mode hotspots

offline-bundle: build
	./scripts/build-offline-bundle.sh --output-dir dist

docker:
	docker build -t punchtrunk:local .

sign:
	# Example: cosign sign --keyless punchtrunk:local
	@echo "Use cosign to sign your image (keyless OIDC supported)."

