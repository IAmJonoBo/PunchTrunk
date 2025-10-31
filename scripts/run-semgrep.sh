#!/usr/bin/env bash
# Run Semgrep with the repository's bundled rule set without reaching out to the network.
# Requires Trunk CLI to provide the semgrep binary (downloaded once into the cache or via offline bundle).

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RULES_DIR="${ROOT_DIR}/semgrep"
REPORT_DIR="${ROOT_DIR}/reports"
OUTPUT_PATH="${1:-${REPORT_DIR}/semgrep.sarif}"

if [[ ! -d ${RULES_DIR} ]]; then
	printf "error: expected Semgrep rules under %s\n" "${RULES_DIR}" >&2
	exit 1
fi

mkdir -p "${REPORT_DIR}"

TEMP_SARIF="$(mktemp "${TMPDIR:-/tmp}/semgrep.XXXXXX.sarif")"
trap 'rm -f "${TEMP_SARIF}"' EXIT

SEMGREP_BIN="${SEMGREP_BIN:-semgrep}"
if ! command -v "${SEMGREP_BIN}" >/dev/null 2>&1; then
	printf "error: semgrep binary not found (set SEMGREP_BIN to override)\n" >&2
	exit 1
fi

"${SEMGREP_BIN}" \
	--config "${RULES_DIR}" \
	--disable-version-check \
	--no-rewrite-rule-ids \
	--metrics=off \
	--error \
	--sarif \
	--output "${TEMP_SARIF}" \
	"${ROOT_DIR}"

cat "${TEMP_SARIF}" >"${OUTPUT_PATH}"
printf "Semgrep analysis complete. Report written to %s\n" "${OUTPUT_PATH}"
