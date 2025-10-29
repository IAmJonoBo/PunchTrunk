#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_NAME="$(basename "$0")"
cd "$ROOT_DIR"

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
  --force                    Overwrite an existing archive with the same name
  -h, --help                 Show this help text
EOF
}

trunk_exec="trunk"
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN* | Windows_NT) trunk_exec="trunk.exe" ;;
*) trunk_exec="trunk" ;;
esac

OUTPUT_DIR="${ROOT_DIR}/dist"
BUNDLE_NAME=""
PUNCHTRUNK_BINARY="bin/punchtrunk"
TRUNK_BINARY="${HOME}/.trunk/bin/${trunk_exec}"
CACHE_DIR="${HOME}/.cache/trunk"
CONFIG_DIR="${ROOT_DIR}/.trunk"
INCLUDE_CACHE=1
FORCE=0

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

PUNCHTRUNK_BINARY="$(abspath "$PUNCHTRUNK_BINARY")"
TRUNK_BINARY="$(abspath "$TRUNK_BINARY")"
CONFIG_DIR="$(abspath "$CONFIG_DIR")"
CACHE_DIR="$(abspath "$CACHE_DIR")"
OUTPUT_DIR="$(abspath "$OUTPUT_DIR")"

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
	printf "hint: run 'trunk init' or pass --trunk-binary\n" >&2
	exit 1
fi

if [[ ! -d $CONFIG_DIR ]]; then
	printf "error: trunk config directory not found at %s\n" "$CONFIG_DIR" >&2
	exit 1
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
trap 'rm -rf "$workdir"' EXIT

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
  "cache_relative_path": "trunk/cache"
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

Usage:
1. Extract this archive on the target host.
2. Export PATH entries:
    export PUNCHTRUNK_HOME="\$(pwd)/${bundle_root_name}"
    export PATH="\${PUNCHTRUNK_HOME}/bin:\${PUNCHTRUNK_HOME}/trunk/bin:\${PATH}"
3. For air-gapped runs, add:
    export PUNCHTRUNK_AIRGAPPED=1
    export PUNCHTRUNK_TRUNK_BINARY="\${PUNCHTRUNK_HOME}/trunk/bin/${trunk_exec}"
4. Run PunchTrunk with your desired modes.

Checksums listed in checksums.txt can be verified with sha256sum or shasum -a 256.
EOF

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

printf "Bundle created: %s\n" "$OUTPUT_PATH"
printf "Bundle checksum: %s  %s\n" "$bundle_hash" "$(basename "$OUTPUT_PATH")"
