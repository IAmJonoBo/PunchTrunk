#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_NAME="$(basename "$0")"
cd "$ROOT_DIR"

HYDRATE_WARNINGS_TEXT=""
HYDRATE_WARNINGS_COUNT=0

usage() {
	cat <<EOF
Usage: ${SCRIPT_NAME} [options]

Build a tarball that bundles PunchTrunk, a Trunk CLI binary, optional cached toolchain
artifacts, and Trunk configuration for air-gapped environments.

Options:
  --output-dir <path>        Directory for generated artifacts (default: ${ROOT_DIR}/dist)
  --bundle-name <name>       Name of the archive to produce (default: punchtrunk-offline-<os>-<arch>.tar.gz)
  --punchtrunk-binary <path> Path to the PunchTrunk binary to include (default: bin/punchtrunk)
  --trunk-binary <path>      Path to the trunk executable to include (default: ${HOME}/.trunk/bin/trunk)
  --cache-dir <path>         Trunk cache directory to bundle (default: ${HOME}/.cache/trunk when present)
  --config-dir <path>        Trunk configuration directory to bundle (default: ${ROOT_DIR}/.trunk)
	--no-cache                 Skip bundling the Trunk cache directory
	--skip-hydrate             Skip prefetching Trunk tool caches before packaging
  --force                    Overwrite an existing archive with the same name
  -h, --help                 Show this help text
EOF
}

# Defaults for target platform
TARGET_OS=""
TARGET_ARCH=""
trunk_exec="trunk"
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN* | Windows_NT)
	trunk_exec="trunk.exe"
	TARGET_OS="windows"
	;;
Darwin*)
	trunk_exec="trunk"
	TARGET_OS="darwin"
	;;
Linux*)
	trunk_exec="trunk"
	TARGET_OS="linux"
	;;
*)
	trunk_exec="trunk"
	;;
esac
TARGET_ARCH="$(uname -m | tr '[:upper:]' '[:lower:]')"
if [[ $TARGET_ARCH == "x86_64" || $TARGET_ARCH == "amd64" ]]; then
	TARGET_ARCH="amd64"
elif [[ $TARGET_ARCH == "aarch64" || $TARGET_ARCH == "arm64" ]]; then
	TARGET_ARCH="arm64"
fi

OUTPUT_DIR="${ROOT_DIR}/dist"
BUNDLE_NAME=""
PUNCHTRUNK_BINARY="bin/punchtrunk"
TRUNK_BINARY="${HOME}/.trunk/bin/${trunk_exec}"
TRUNK_BINARY_USER_SUPPLIED=0
CACHE_DIR="${HOME}/.cache/trunk"
CONFIG_DIR="${ROOT_DIR}/.trunk"
INCLUDE_CACHE=1
FORCE=0
HYDRATE=1

while [[ $# -gt 0 ]]; do
	case "$1" in
	--output-dir)
		OUTPUT_DIR="$2"
		shift 2
		;;
	--bundle-name)
		BUNDLE_NAME="$2"
		shift 2
		;;
	--punchtrunk-binary)
		PUNCHTRUNK_BINARY="$2"
		shift 2
		;;
        --trunk-binary)
                TRUNK_BINARY="$2"
                TRUNK_BINARY_USER_SUPPLIED=1
                shift 2
                ;;
	--target-os)
		TARGET_OS="$2"
		shift 2
		;;
	--target-arch)
		TARGET_ARCH="$2"
		shift 2
		;;
	--cache-dir)
		CACHE_DIR="$2"
		shift 2
		;;
	--config-dir)
		CONFIG_DIR="$2"
		shift 2
		;;
	--no-cache)
		INCLUDE_CACHE=0
		shift
		;;
	--skip-hydrate)
		HYDRATE=0
		shift
		;;
	--force)
		FORCE=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf "Unknown option: %s\n\n" "$1" >&2
		usage >&2
		exit 1
		;;
	esac
done

abspath() {
	if [[ $1 == /* ]]; then
		printf "%s\n" "$1"
	else
		printf "%s/%s\n" "$PWD" "$1"
	fi
}

json_escape() {
	local value="$1"
	value="${value//\\/\\\\}"
	value="${value//\"/\\\"}"
	value="${value//$'\n'/\\n}"
	printf '%s' "$value"
}

record_hydrate_warning() {
	local message="$1"
	if [[ -z $message ]]; then
		return
	fi
	if [[ -z $HYDRATE_WARNINGS_TEXT ]]; then
		HYDRATE_WARNINGS_TEXT="$message"
	else
		HYDRATE_WARNINGS_TEXT+=$'\n'"$message"
	fi
	HYDRATE_WARNINGS_COUNT=$((HYDRATE_WARNINGS_COUNT + 1))
}

hydrate_warnings_json() {
	if [[ -z $HYDRATE_WARNINGS_TEXT ]]; then
		printf '[]'
		return
	fi
	local first=1
	printf '['
	while IFS= read -r line; do
		if [[ -z $line ]]; then
			continue
		fi
		if ((first)); then
			first=0
		else
			printf ','
		fi
		printf '"%s"' "$(json_escape "$line")"
	done <<<"$HYDRATE_WARNINGS_TEXT"
	printf ']'
}

compute_sha256() {
	local target="$1"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$target" | awk '{print $1}'
		return 0
	fi
	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$target" | awk '{print $1}'
		return 0
	fi
	printf "error: neither sha256sum nor shasum is available\n" >&2
	return 1
}

run_hydrate_command() {
	local description="$1"
	shift
	local output
	set +e
	output=$(TRUNK_CONFIG_DIR="$CONFIG_DIR" TRUNK_CACHE_DIR="$CACHE_DIR" "$@" 2>&1)
	local rc=$?
	set -e
	if [[ -n ${hydrate_log-} ]]; then
		printf '%s\n' "$output" >>"$hydrate_log"
	fi
	if [[ $rc -ne 0 ]]; then
		if [[ $HYDRATE_STATUS == "success" ]]; then
			HYDRATE_STATUS="partial"
		fi
		local tail_msg
		tail_msg=$(printf '%s\n' "$output" | tail -n 1)
		record_hydrate_warning "$description failed (exit $rc): $tail_msg"
	fi
}

PUNCHTRUNK_BINARY="$(abspath "$PUNCHTRUNK_BINARY")"
CONFIG_DIR="$(abspath "$CONFIG_DIR")"
CACHE_DIR="$(abspath "$CACHE_DIR")"
OUTPUT_DIR="$(abspath "$OUTPUT_DIR")"

TRUNK_CLI_VERSION=""
if [[ -f "$CONFIG_DIR/trunk.yaml" ]]; then
	TRUNK_CLI_VERSION=$(awk '
		/^[[:space:]]*cli:/ { in_cli=1; next }
		in_cli && /^[^[:space:]]/ { in_cli=0 }
		in_cli && /^[[:space:]]*version:/ {
			sub(/^[[:space:]]*version:[[:space:]]*/, "", $0)
			gsub(/"/, "", $0)
			gsub(/[[:space:]]+$/, "", $0)
			print $0
			exit
		}
	' "$CONFIG_DIR/trunk.yaml")
	TRUNK_CLI_VERSION="${TRUNK_CLI_VERSION}" # ensure defined
fi

TRUNK_BINARY_SOURCE="user-supplied"
if [[ $TRUNK_BINARY_USER_SUPPLIED -eq 0 ]]; then
        TRUNK_BINARY_SOURCE="default"
fi

download_tmp=""

maybe_download_trunk() {
        local trunk_version="$1"
        local os="$2"
        local arch="$3"
        local trunk_tmp="$(mktemp -d)"
        download_tmp="$trunk_tmp"
        local trunk_url=""
        local resolved_arch="$arch"
        if [[ $os == "darwin" && $resolved_arch == "amd64" ]]; then
                resolved_arch="x86_64"
        elif [[ $os == "linux" && $resolved_arch == "amd64" ]]; then
                resolved_arch="x86_64"
        elif [[ $os == "linux" && $resolved_arch == "arm64" ]]; then
                resolved_arch="arm64"
        fi
        if [[ $os == "windows" ]]; then
                trunk_url="https://trunk.io/releases/${trunk_version}/trunk-${trunk_version}-windows-x86_64.zip"
                trunk_exec="trunk.exe"
                curl -Ls "$trunk_url" -o "$trunk_tmp/trunk.zip"
                unzip -q "$trunk_tmp/trunk.zip" -d "$trunk_tmp"
                TRUNK_BINARY="$trunk_tmp/trunk.exe"
        else
                trunk_url="https://trunk.io/releases/${trunk_version}/trunk-${trunk_version}-${os}-${resolved_arch}.tar.gz"
                curl -Ls "$trunk_url" | tar -xz -C "$trunk_tmp"
                TRUNK_BINARY="$trunk_tmp/trunk"
        fi
}

if [[ $TRUNK_BINARY_USER_SUPPLIED -eq 0 && ! -f $TRUNK_BINARY ]]; then
        TRUNK_VERSION="${TRUNK_CLI_VERSION:-1.25.0}"
        maybe_download_trunk "$TRUNK_VERSION" "$TARGET_OS" "$TARGET_ARCH"
        TRUNK_BINARY_SOURCE="auto-downloaded"
fi

if [[ -z $TRUNK_CLI_VERSION && -n ${TRUNK_VERSION-} ]]; then
        TRUNK_CLI_VERSION="$TRUNK_VERSION"
fi

TRUNK_BINARY="$(abspath "$TRUNK_BINARY")"

if [[ ! -f $PUNCHTRUNK_BINARY ]]; then
	printf "error: PunchTrunk binary not found at %s\n" "$PUNCHTRUNK_BINARY" >&2
	printf "hint: run 'make build' or pass --punchtrunk-binary\n" >&2
	exit 1
fi
if [[ ! -x $PUNCHTRUNK_BINARY ]]; then
	chmod +x "$PUNCHTRUNK_BINARY"
fi

if [[ ! -f $TRUNK_BINARY ]]; then
        printf "error: trunk binary not found at %s\n" "$TRUNK_BINARY" >&2
        printf "hint: run 'trunk init', pass --trunk-binary, or let the script auto-download for --target-os/--target-arch\n" >&2
        exit 1
fi

case "$TRUNK_BINARY_SOURCE" in
user-supplied)
        printf "Using user-supplied trunk binary: %s\n" "$TRUNK_BINARY"
        ;;
auto-downloaded)
        printf "Auto-downloaded trunk binary for %s/%s -> %s\n" "$TARGET_OS" "$TARGET_ARCH" "$TRUNK_BINARY"
        ;;
*)
        printf "Using existing trunk binary: %s\n" "$TRUNK_BINARY"
        ;;
esac

if [[ ! -d $CONFIG_DIR ]]; then
	printf "error: trunk config directory not found at %s\n" "$CONFIG_DIR" >&2
	exit 1
fi

if [[ $INCLUDE_CACHE -eq 0 ]]; then
	HYDRATE=0
fi

HYDRATE_STATUS="skipped"
HYDRATE_WARNINGS_TEXT=""
HYDRATE_WARNINGS_COUNT=0
HYDRATE_ATTEMPTED=false
hydrate_log=""

if [[ $HYDRATE -eq 1 ]]; then
	HYDRATE_ATTEMPTED=true
	HYDRATE_STATUS="success"
	if [[ ! -d $CACHE_DIR ]]; then
		mkdir -p "$CACHE_DIR"
	fi
	if [[ ! -d $CACHE_DIR ]]; then
		HYDRATE_STATUS="partial"
		record_hydrate_warning "cache directory $CACHE_DIR could not be created"
	else
		hydrate_log=$(mktemp "${TMPDIR:-/tmp}/punchtrunk-hydrate.XXXXXX")
		run_hydrate_command "trunk install" "$TRUNK_BINARY" install --ci
	fi
fi

mkdir -p "$OUTPUT_DIR"

if [[ -z $BUNDLE_NAME ]]; then
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m | tr '[:upper:]' '[:lower:]')"
	BUNDLE_NAME="punchtrunk-offline-${os}-${arch}.tar.gz"
fi

OUTPUT_PATH="${OUTPUT_DIR}/${BUNDLE_NAME}"
if [[ -f $OUTPUT_PATH && $FORCE -eq 0 ]]; then
	printf "error: %s already exists (use --force to overwrite)\n" "$OUTPUT_PATH" >&2
	exit 1
fi

workdir="$(mktemp -d "${TMPDIR:-/tmp}/punchtrunk-offline.XXXXXX")"
trap 'rm -rf "$workdir"; if [[ -n "${hydrate_log:-}" && -f "$hydrate_log" ]]; then rm -f "$hydrate_log"; fi; if [[ -n "${download_tmp:-}" && -d "$download_tmp" ]]; then rm -rf "$download_tmp"; fi' EXIT

bundle_root_name="${BUNDLE_NAME%.tar.gz}"
if [[ $bundle_root_name == "$BUNDLE_NAME" ]]; then
	bundle_root_name="${BUNDLE_NAME%.tgz}"
fi
if [[ $bundle_root_name == "$BUNDLE_NAME" ]]; then
	bundle_root_name="punchtrunk-offline"
fi

bundle_root="${workdir}/${bundle_root_name}"
mkdir -p "${bundle_root}/bin"
mkdir -p "${bundle_root}/trunk/bin"
mkdir -p "${bundle_root}/trunk/cache"
mkdir -p "${bundle_root}/trunk/config"

cp "$PUNCHTRUNK_BINARY" "${bundle_root}/bin/punchtrunk"
cp "$TRUNK_BINARY" "${bundle_root}/trunk/bin/${trunk_exec}"

copy_directory() {
	local source_dir="$1"
	local target_dir="$2"
	mkdir -p "$target_dir"
	if [[ ! -d $source_dir ]]; then
		return
	fi
	if [[ -z "$(ls -A "$source_dir" 2>/dev/null)" ]]; then
		return
	fi
	tar -C "$source_dir" -cf - . | tar -C "$target_dir" -xf -
}

copy_directory "$CONFIG_DIR" "${bundle_root}/trunk/config"

CACHE_INCLUDED="false"
if [[ $INCLUDE_CACHE -eq 1 && -d $CACHE_DIR ]]; then
	copy_directory "$CACHE_DIR" "${bundle_root}/trunk/cache"
	CACHE_INCLUDED="true"
fi

trunk_version="unknown"
if version_output=$("${bundle_root}/trunk/bin/${trunk_exec}" --version 2>&1 | head -n 1); then
	trunk_version="$version_output"
fi

CONFIG_SHA=""
if [[ -f "$CONFIG_DIR/trunk.yaml" ]]; then
	if ! CONFIG_SHA=$(compute_sha256 "$CONFIG_DIR/trunk.yaml"); then
		CONFIG_SHA=""
	fi
fi
HYDRATE_WARNINGS_JSON=$(hydrate_warnings_json)
if [[ $HYDRATE_STATUS == "success" || $HYDRATE_STATUS == "partial" ]]; then
	HYDRATE_ATTEMPTED_JSON=true
else
	HYDRATE_ATTEMPTED_JSON=false
fi
CACHE_DIR_SOURCE_ESC=$(json_escape "$CACHE_DIR")
TRUNK_CLI_VERSION_ESC=$(json_escape "$TRUNK_CLI_VERSION")
CONFIG_SHA_ESC=$(json_escape "$CONFIG_SHA")
HYDRATE_STATUS_ESC=$(json_escape "$HYDRATE_STATUS")

manifest_path="${bundle_root}/manifest.json"
created_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
cat >"$manifest_path" <<EOF
{
  "created_at": "${created_at}",
  "punchtrunk_binary": "$(basename "$PUNCHTRUNK_BINARY")",
  "trunk_binary": "$(basename "$TRUNK_BINARY")",
  "trunk_version": "${trunk_version}",
  "cache_included": ${CACHE_INCLUDED},
  "config_relative_path": "trunk/config",
	"cache_relative_path": "trunk/cache",
	"trunk_cli_version": "${TRUNK_CLI_VERSION_ESC}",
	"trunk_config_sha256": "${CONFIG_SHA_ESC}",
	"hydrate_attempted": ${HYDRATE_ATTEMPTED_JSON},
	"hydrate_status": "${HYDRATE_STATUS_ESC}",
	"hydrate_warnings": ${HYDRATE_WARNINGS_JSON},
	"cache_dir_source": "${CACHE_DIR_SOURCE_ESC}"
}
EOF

readme_path="${bundle_root}/README.txt"
cat >"$readme_path" <<EOF
PunchTrunk Offline Bundle
=========================

Contents:
- bin/punchtrunk: PunchTrunk CLI binary.
- trunk/bin/trunk: Trunk CLI executable.
- trunk/config: Repository Trunk configuration for fast bootstrap.
- trunk/cache: Optional cached toolchain assets (present when generated on a machine with Trunk cache).
- manifest.json: Metadata about the bundle.
- checksums.txt: SHA-256 checksums for bundle contents.
- punchtrunk-airgap.env / punchtrunk-airgap.ps1: Convenience environment exports for POSIX shells and PowerShell.

Usage:
1. Extract this archive on the target host.
2. Source the environment helper for your shell:
	# POSIX shells (bash, zsh)
	source ./punchtrunk-airgap.env

	# PowerShell
	. ./punchtrunk-airgap.ps1
3. Run PunchTrunk with your desired modes.

Checksums listed in checksums.txt can be verified with sha256sum or shasum -a 256.
EOF

airgap_env_path="${bundle_root}/punchtrunk-airgap.env"
cat >"$airgap_env_path" <<EOF
# shellcheck shell=bash
__punchtrunk_bundle_dir="\$(cd "\$(dirname "\${BASH_SOURCE[0]:-$0}")" && pwd)"
if [[ -z "\${PUNCHTRUNK_HOME:-}" ]]; then
	export PUNCHTRUNK_HOME="\${__punchtrunk_bundle_dir}"
fi
export PUNCHTRUNK_TRUNK_BINARY="\${PUNCHTRUNK_HOME}/trunk/bin/${trunk_exec}"
export PUNCHTRUNK_AIRGAPPED="\${PUNCHTRUNK_AIRGAPPED:-1}"
__punchtrunk_bin="\${PUNCHTRUNK_HOME}/bin"
__punchtrunk_trunk="\${PUNCHTRUNK_HOME}/trunk/bin"
case ":\${PATH}:" in
	*":\${__punchtrunk_bin}:"*) ;;
	*) PATH="\${__punchtrunk_bin}:\${PATH}" ;;
esac
case ":\${PATH}:" in
	*":\${__punchtrunk_trunk}:"*) ;;
	*) PATH="\${__punchtrunk_trunk}:\${PATH}" ;;
esac
export PATH
unset __punchtrunk_bin __punchtrunk_trunk
unset __punchtrunk_bundle_dir
EOF

airgap_ps1_path="${bundle_root}/punchtrunk-airgap.ps1"
cat >"$airgap_ps1_path" <<EOF
# PowerShell environment helper for PunchTrunk offline bundles


\$bundleDir = Split-Path -Parent \$MyInvocation.MyCommand.Definition
if (-not \$env:PUNCHTRUNK_HOME) {
	\$env:PUNCHTRUNK_HOME = \$bundleDir
}
\$env:PUNCHTRUNK_TRUNK_BINARY = Join-Path \$env:PUNCHTRUNK_HOME "trunk/bin/${trunk_exec}"
if (-not \$env:PUNCHTRUNK_AIRGAPPED) {
	\$env:PUNCHTRUNK_AIRGAPPED = "1"
}
\$binPath = Join-Path \$env:PUNCHTRUNK_HOME "bin"
\$trunkPath = Join-Path \$env:PUNCHTRUNK_HOME "trunk/bin"
\$orderedPaths = @()
foreach (\$p in @(\$binPath, \$trunkPath) + (\$env:PATH -split ';')) {
	if ([string]::IsNullOrWhiteSpace(\$p)) {
		continue
	}
	if (-not (\$orderedPaths -contains \$p)) {
		\$orderedPaths += \$p
	}
}
\$env:PATH = (\$orderedPaths -join ';')
EOF

checksums_path="${bundle_root}/checksums.txt"
: >"$checksums_path"
while IFS= read -r file; do
	hash="$(compute_sha256 "$file")" || {
		printf "error: unable to compute checksum for %s\n" "$file" >&2
		exit 1
	}
	rel_path="${file#${bundle_root}/}"
	printf "%s  %s\n" "$hash" "$rel_path" >>"$checksums_path"
done < <(LC_ALL=C find "$bundle_root" -type f ! -name 'checksums.txt' -print | sort)

mkdir -p "$OUTPUT_DIR"
tar -C "$workdir" -czf "$OUTPUT_PATH" "$bundle_root_name"

bundle_hash="$(compute_sha256 "$OUTPUT_PATH")" || {
	printf "error: unable to compute checksum for %s\n" "$OUTPUT_PATH" >&2
	exit 1
}
printf "%s  %s\n" "$bundle_hash" "$(basename "$OUTPUT_PATH")" >"${OUTPUT_PATH}.sha256"

if [[ $HYDRATE_WARNINGS_COUNT -gt 0 ]]; then
	printf "warning: hydration encountered issues while preparing caches:\n" >&2
	while IFS= read -r warn; do
		if [[ -z $warn ]]; then
			continue
		fi
		printf "  - %s\n" "$warn" >&2
	done <<<"$HYDRATE_WARNINGS_TEXT"
fi

printf "Bundle created: %s\n" "$OUTPUT_PATH"
printf "Bundle checksum: %s  %s\n" "$bundle_hash" "$(basename "$OUTPUT_PATH")"
