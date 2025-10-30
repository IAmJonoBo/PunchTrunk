#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

BASE_BRANCH="origin/main"
MODES="fmt,lint,hotspots"
PUNCH_BINARY="./bin/punchtrunk"
CONFIG_DIR=".trunk"
CACHE_DIR="${HOME}/.cache/trunk"
TRUNK_BINARY=""
SKIP_NETWORK=0
ONLY_PREP=0
PASSTHRU=()

log() {
	printf '[quality-suite] %s\n' "$1"
}

usage() {
	cat <<'EOF'
Usage: run-quality-suite.sh [options] [-- <additional punchtrunk args>]

Run PunchTrunk's recommended offline quality checks. The script invokes
prep-runner.sh to hydrate Trunk caches and produce tool-health reports,
then executes the configured PunchTrunk modes.

Options:
  --base-branch <ref>      Base branch for hotspots diffing (default: origin/main)
  --modes <list>           Comma-separated PunchTrunk modes (default: fmt,lint,hotspots)
  --punchtrunk <path>      Path to PunchTrunk binary (default: ./bin/punchtrunk)
  --config-dir <path>      Trunk configuration directory (default: .trunk)
  --cache-dir <path>       Trunk cache directory (default: $HOME/.cache/trunk)
  --trunk-binary <path>    Explicit trunk executable to reuse
  --skip-network-check     Skip outbound network probe in prep-runner
  --prep-only              Run prep-runner and exit without invoking PunchTrunk
  -h, --help               Show this help message

Pass any additional PunchTrunk flags after --. They will be appended to the
punchtrunk invocation.
EOF
}

abspath() {
	if [[ $# -eq 0 ]]; then
		return 1
	fi
	if [[ -d $1 ]]; then
		local dirpath
		dirpath="$(cd "$1" && pwd)"
		printf '%s\n' "${dirpath}"
		return 0
	fi
	local dir base dirpath
	dir=$(dirname "$1")
	base=$(basename "$1")
	dirpath="$(cd "${dir}" && pwd)"
	printf '%s/%s\n' "${dirpath}" "${base}"
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--base-branch)
		BASE_BRANCH="$2"
		shift 2
		;;
	--modes)
		MODES="$2"
		shift 2
		;;
	--punchtrunk)
		PUNCH_BINARY="$2"
		shift 2
		;;
	--config-dir)
		CONFIG_DIR="$2"
		shift 2
		;;
	--cache-dir)
		CACHE_DIR="$2"
		shift 2
		;;
	--trunk-binary)
		TRUNK_BINARY="$2"
		shift 2
		;;
	--skip-network-check)
		SKIP_NETWORK=1
		shift
		;;
	--prep-only)
		ONLY_PREP=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	--)
		shift
		PASSTHRU+=("$@")
		break
		;;
	*)
		PASSTHRU+=("$1")
		shift
		;;
	esac
done

PREP_SCRIPT="${ROOT_DIR}/scripts/prep-runner.sh"
if [[ ! -x ${PREP_SCRIPT} ]]; then
	log "Making prep-runner executable"
	chmod +x "${PREP_SCRIPT}"
fi

PUNCH_ABS="$(abspath "${PUNCH_BINARY}")"
if [[ ! -x ${PUNCH_ABS} ]]; then
	log "PunchTrunk binary not found at ${PUNCH_ABS}"
	log "Run 'make build' or provide --punchtrunk <path>"
	exit 1
fi

PREP_CMD=("bash" "${PREP_SCRIPT}" "--config-dir" "${CONFIG_DIR}" "--cache-dir" "${CACHE_DIR}" "--punchtrunk" "${PUNCH_ABS}")
if [[ -n ${TRUNK_BINARY} ]]; then
	PREP_CMD+=("--trunk-binary" "${TRUNK_BINARY}")
fi
if [[ ${SKIP_NETWORK} -eq 1 ]]; then
	PREP_CMD+=("--skip-network-check")
fi

log "Running prep-runner"
"${PREP_CMD[@]}"

if [[ ${ONLY_PREP} -eq 1 ]]; then
	log "Prep-only mode enabled; skipping PunchTrunk execution"
	exit 0
fi

RUN_CMD=("${PUNCH_ABS}" "--mode" "${MODES}" "--base-branch" "${BASE_BRANCH}")
if [[ -n ${CONFIG_DIR} ]]; then
	RUN_CMD+=("--trunk-config-dir" "${CONFIG_DIR}")
fi
if [[ -n ${TRUNK_BINARY} ]]; then
	RUN_CMD+=("--trunk-binary" "${TRUNK_BINARY}")
fi
if [[ ${#PASSTHRU[@]} -gt 0 ]]; then
	RUN_CMD+=("${PASSTHRU[@]}")
fi

log "Running PunchTrunk modes: ${MODES}"
"${RUN_CMD[@]}"

log "Quality suite finished"
