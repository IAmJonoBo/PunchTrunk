SHELL := /bin/bash
BIN := bin/punchtrunk
VERSION ?= dev
LDFLAGS := -s -w -X main.Version=$(VERSION)

<<<<<<< HEAD
.PHONY: help build run fmt lint hotspots test offline-bundle eval-hotspots security validate-env prep-runner
=======
.PHONY: build run fmt lint hotspots test offline-bundle eval-hotspots semgrep
>>>>>>> 9b5cdee (feat: add Semgrep integration and update build scripts; enhance QA documentation)

# Default target: show help
.DEFAULT_GOAL := help

help: ## Display this help message
	@echo "PunchTrunk Makefile - Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Common workflows:"
	@echo "  make build test        # Build and test"
	@echo "  make prep-runner run   # Prepare environment and run PunchTrunk"
	@echo "  make validate-env      # Check if environment is ready"
	@echo ""

build: ## Build the PunchTrunk binary
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/punchtrunk

run: build ## Build and run PunchTrunk with default modes (fmt,lint,hotspots)
	$(BIN) --mode fmt,lint,hotspots

fmt: ## Format code using Trunk (requires Trunk CLI)
	trunk fmt

lint: ## Lint code using Trunk (requires Trunk CLI)
	trunk check

test: ## Run all tests (Go tests + BATS tests if available)
	go test -v ./...
	@if command -v bats >/dev/null 2>&1; then \
		bats scripts/tests; \
	else \
		echo "bats not found; skipping shell tests" >&2; \
	fi

hotspots: build ## Compute and export hotspots to SARIF
	$(BIN) --mode hotspots

eval-hotspots: build ## Evaluate hotspots with scoring details
	./scripts/eval-hotspots.sh

<<<<<<< HEAD
offline-bundle: build ## Build offline bundle for air-gapped environments
	./scripts/build-offline-bundle.sh --output-dir dist

security: ## Run Semgrep security scan (requires semgrep)
	@if [ ! -f semgrep/offline-ci.yml ]; then \
		echo "Error: semgrep config not found at semgrep/offline-ci.yml" >&2; \
		exit 1; \
	fi
	@if command -v semgrep >/dev/null 2>&1; then \
		semgrep --config=semgrep/offline-ci.yml --metrics=off .; \
	else \
		echo "semgrep not found; install via 'pip install semgrep' or skip security scan" >&2; \
		exit 1; \
	fi

validate-env: ## Validate that the environment has all required tools
	@echo "Validating environment for PunchTrunk..."
	@bash scripts/validate-agent-environment.sh || true

prep-runner: build ## Prepare runner by hydrating caches and running health checks
	@echo "Preparing runner environment..."
	@bash scripts/prep-runner.sh \
		--config-dir=.trunk \
		--cache-dir="$$HOME/.cache/trunk" \
		--punchtrunk=$(BIN) \
		--json-output=reports/preflight.json

=======
semgrep:
	./scripts/run-semgrep.sh

offline-bundle: build
	./scripts/build-offline-bundle.sh --output-dir dist
>>>>>>> 9b5cdee (feat: add Semgrep integration and update build scripts; enhance QA documentation)
