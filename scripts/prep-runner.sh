#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR=".trunk"
CACHE_DIR="${HOME}/.cache/trunk"
PUNCH_BINARY="./bin/punchtrunk"
TRUNK_BINARY=""
JSON_OUTPUT="reports/preflight.json"
SKIP_NETWORK=0
VERBOSE=1

log_info() {
	printf "[prep-runner] %s\n" "$1"
}

log_warn() {
	printf "[prep-runner][warning] %s\n" "$1"
}

log_error() {
	printf "[prep-runner][error] %s\n" "$1" >&2
}

usage() {
	cat <<'EOF'
Usage: prep-runner.sh [options]

Prepare a CI runner for PunchTrunk by hydrating Trunk caches, running
punchtrunk tool-health diagnostics, and recording results.

Options:
  --config-dir <path>        Trunk configuration directory (default: .trunk)
  --cache-dir <path>         Trunk cache directory (default: $HOME/.cache/trunk)
  --punchtrunk <path>        Path to punchtrunk binary (default: ./bin/punchtrunk)
  --trunk-binary <path>      Explicit path to trunk executable
  --json-output <path>       Where to write JSON summary (default: reports/preflight.json)
  --skip-network-check       Skip outbound network probe (assume offline)
  --quiet                    Reduce stdout logging
  -h, --help                 Show this help text
EOF
}

abspath() {
	if [[ $# -eq 0 ]]; then
		return 1
	fi
	if [[ -d $1 ]]; then
		(cd "$1" && pwd)
	else
		local dir
		dir=$(dirname "$1")
		local base
		base=$(basename "$1")
		(cd "$dir" && printf "%s/%s" "$(pwd)" "$base")
	fi
}

json_escape() {
	local value="$1"
	value="${value//\\/\\\\}"
	value="${value//\"/\\\"}"
	value="${value//$'\n'/\\n}"
	printf '%s' "$value"
}

NETWORK_STATUS="skipped"
NETWORK_MESSAGE="network check skipped"
TRUNK_STATUS="skipped"
TRUNK_MESSAGE=""
TOOL_HEALTH_STATUS="skipped"
TOOL_HEALTH_MESSAGE="punchtrunk binary not found"
WARNINGS=()
TOOL_HEALTH_OUTPUT=""
TOOL_HEALTH_SUMMARY=""
TOOL_HEALTH_LOG=""

append_warning() {
	WARNINGS+=("$1")
	log_warn "$1"
}

append_summary() {
	local line="$1"
	if [[ -n ${GITHUB_STEP_SUMMARY-} ]]; then
		printf '%s\n' "$line" >>"$GITHUB_STEP_SUMMARY"
	fi
}

save_env_var() {
	local key="$1"
	local value="$2"
	if [[ -n ${GITHUB_ENV-} ]]; then
		printf '%s=%s\n' "$key" "$value" >>"$GITHUB_ENV"
	fi
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--config-dir)
		CONFIG_DIR="$2"
		shift 2
		;;
	--cache-dir)
		CACHE_DIR="$2"
		shift 2
		;;
	--punchtrunk)
		PUNCH_BINARY="$2"
		shift 2
		;;
	--trunk-binary)
		TRUNK_BINARY="$2"
		shift 2
		;;
	--json-output)
		JSON_OUTPUT="$2"
		shift 2
		;;
	--skip-network-check)
		SKIP_NETWORK=1
		shift
		;;
	--quiet)
		VERBOSE=0
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		log_error "Unknown option: $1"
		usage
		exit 1
		;;
	esac
done

log_info "PunchTrunk runner preparation starting"

if [[ $SKIP_NETWORK -eq 0 ]]; then
	if command -v curl >/dev/null 2>&1; then
		if curl -I --silent --show-error --max-time 5 https://api.github.com >/dev/null 2>&1; then
			NETWORK_STATUS="online"
			NETWORK_MESSAGE="https://api.github.com reachable"
		else
			NETWORK_STATUS="offline"
			NETWORK_MESSAGE="Unable to reach https://api.github.com"
			append_warning "Network probe failed; assuming offline environment"
		fi
	else
		NETWORK_STATUS="unknown"
		NETWORK_MESSAGE="curl not available"
		append_warning "curl not found; skipping network probe"
	fi
else
	NETWORK_STATUS="skipped"
	NETWORK_MESSAGE="Network probe disabled via flag"
fi

if [[ -n ${TRUNK_BINARY-} ]]; then
	if [[ ! -x $TRUNK_BINARY ]]; then
		append_warning "Provided trunk binary $TRUNK_BINARY is not executable"
		TRUNK_BINARY=""
	fi
fi

if [[ -z ${TRUNK_BINARY-} ]]; then
	if [[ -n ${PUNCHTRUNK_TRUNK_BINARY-} && -x ${PUNCHTRUNK_TRUNK_BINARY} ]]; then
		TRUNK_BINARY="${PUNCHTRUNK_TRUNK_BINARY}"
	elif command -v trunk >/dev/null 2>&1; then
		TRUNK_BINARY="$(command -v trunk)"
	elif [[ -x "${HOME}/.trunk/bin/trunk" ]]; then
		TRUNK_BINARY="${HOME}/.trunk/bin/trunk"
	fi
fi

if [[ -z ${TRUNK_BINARY-} ]]; then
	TRUNK_STATUS="missing"
	TRUNK_MESSAGE="trunk binary not found"
	append_warning "Trunk executable not found; skipping cache hydration"
else
	TRUNK_BINARY="$(abspath "$TRUNK_BINARY")"
	if [[ ! -x $TRUNK_BINARY ]]; then
		TRUNK_STATUS="missing"
		TRUNK_MESSAGE="trunk binary not executable"
		append_warning "Trunk executable $TRUNK_BINARY is not executable"
	else
		if [[ $VERBOSE -eq 1 ]]; then
			log_info "Using trunk binary: $TRUNK_BINARY"
		fi
		mkdir -p "$CACHE_DIR"
		mkdir -p "$CONFIG_DIR"
		TRUNK_CACHE_DIR="$(abspath "$CACHE_DIR")"
		TRUNK_CONFIG_DIR="$(abspath "$CONFIG_DIR")"
		export TRUNK_CACHE_DIR
		export TRUNK_CONFIG_DIR
		save_env_var "TRUNK_CACHE_DIR" "$TRUNK_CACHE_DIR"
		save_env_var "TRUNK_CONFIG_DIR" "$TRUNK_CONFIG_DIR"
		save_env_var "TRUNK_BINARY" "$TRUNK_BINARY"
		TRUNK_STATUS="attempted"
		TRUNK_MESSAGE="Hydrating caches via trunk install"
		if ! "$TRUNK_BINARY" install --ci >/dev/null 2>&1; then
			append_warning "trunk install failed; caches may be incomplete"
			TRUNK_STATUS="partial"
			TRUNK_MESSAGE="trunk install failed"
		else
			TRUNK_STATUS="success"
			TRUNK_MESSAGE="Trunk caches hydrated"
		fi
	fi
fi

if [[ -x $PUNCH_BINARY ]]; then
	PUNCH_BINARY="$(abspath "$PUNCH_BINARY")"
	TOOL_HEALTH_STATUS="attempted"
	TOOL_HEALTH_MESSAGE="Running punchtrunk tool-health"
	mkdir -p reports
	TOOL_HEALTH_OUTPUT="reports/tool-health-preflight.json"
	TOOL_HEALTH_SUMMARY="reports/tool-health-preflight.md"
	TOOL_HEALTH_LOG="reports/tool-health-preflight.log"
	if ! "$PUNCH_BINARY" --mode tool-health --trunk-config-dir "$CONFIG_DIR" --trunk-binary "$TRUNK_BINARY" --tool-health-format summary --tool-health-json "${TOOL_HEALTH_OUTPUT}" >"$TOOL_HEALTH_SUMMARY" 2>"${TOOL_HEALTH_LOG}"; then
		TOOL_HEALTH_STATUS="failed"
		TOOL_HEALTH_MESSAGE="punchtrunk tool-health returned non-zero"
		append_warning "punchtrunk tool-health reported issues"
	else
		TOOL_HEALTH_STATUS="success"
		if [[ $VERBOSE -eq 1 ]]; then
			log_info "tool-health summary saved to $TOOL_HEALTH_SUMMARY"
		fi
	fi
else
	TOOL_HEALTH_STATUS="skipped"
	TOOL_HEALTH_MESSAGE="punchtrunk binary not present"
	append_warning "punchtrunk binary not found at $PUNCH_BINARY; skipping tool-health"
fi

timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
mkdir -p "$(dirname "$JSON_OUTPUT")"
warnings_json="[]"
if [[ ${#WARNINGS[@]} -gt 0 ]]; then
	warning_entries=""
	for warn in "${WARNINGS[@]}"; do
		escaped_value=$(json_escape "$warn")
		escaped="\"${escaped_value}\""
		if [[ -z $warning_entries ]]; then
			warning_entries="$escaped"
		else
			warning_entries+=",$escaped"
		fi
	done
	warnings_json="[${warning_entries}]"
fi

network_message_esc=$(json_escape "${NETWORK_MESSAGE}")
trunk_message_esc=$(json_escape "${TRUNK_MESSAGE}")
cache_dir_esc=$(json_escape "${TRUNK_CACHE_DIR-}")
tool_health_message_esc=$(json_escape "${TOOL_HEALTH_MESSAGE}")
tool_health_output_esc=$(json_escape "${TOOL_HEALTH_OUTPUT}")
tool_health_summary_esc=$(json_escape "${TOOL_HEALTH_SUMMARY}")
tool_health_log_esc=$(json_escape "${TOOL_HEALTH_LOG}")

cat >"$JSON_OUTPUT" <<EOF
{
  "timestamp": "${timestamp}",
  "network": {
    "status": "${NETWORK_STATUS}",
		"message": "${network_message_esc}"
  },
  "trunk_fetch": {
    "status": "${TRUNK_STATUS}",
		"message": "${trunk_message_esc}",
		"cache_dir": "${cache_dir_esc}"
  },
  "tool_health": {
    "status": "${TOOL_HEALTH_STATUS}",
		"message": "${tool_health_message_esc}",
		"output": "${tool_health_output_esc}",
		"summary": "${tool_health_summary_esc}",
		"log": "${tool_health_log_esc}"
  },
  "warnings": ${warnings_json}
}
EOF

if [[ $VERBOSE -eq 1 ]]; then
	log_info "Summary written to $JSON_OUTPUT"
fi

append_summary "### PunchTrunk runner prep"
append_summary "- Network: ${NETWORK_STATUS} (${NETWORK_MESSAGE})"
append_summary "- Trunk: ${TRUNK_STATUS}"
append_summary "- Tool health: ${TOOL_HEALTH_STATUS}"
if [[ -n ${TOOL_HEALTH_SUMMARY} ]]; then
	append_summary "- Tool health summary: ${TOOL_HEALTH_SUMMARY}"
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
	append_summary "- Warnings:"
	for warn in "${WARNINGS[@]}"; do
		append_summary "  - ${warn}"
	done
fi

log_info "Runner preparation complete"
exit 0
