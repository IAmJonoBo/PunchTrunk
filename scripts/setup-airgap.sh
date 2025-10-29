#!/usr/bin/env bash
set -euo pipefail

SCRIPT_NAME="$(basename "$0")"

usage() {
	cat <<EOF
Usage: ${SCRIPT_NAME} --bundle <path> [options]

Prepare an air-gapped PunchTrunk installation by extracting an offline bundle,
creating stable symlinks, wiring cache directories, and emitting a shell
fragment with the required environment variables.

Required:
  --bundle <path>          Path to a PunchTrunk offline bundle tarball (.tar.gz)

Options:
  --install-dir <path>     Target directory for the installation (default: /opt/punchtrunk)
  --env-file <path>        Where to write the environment export script (default: <install-dir>/punchtrunk-airgap.env)
  --checksum <path>        Optional SHA-256 checksum file produced beside the bundle
  --no-cache-link          Skip creating \$HOME/.cache/trunk symlink
  --force                  Overwrite existing installation and cache links when present
  -h, --help               Show this help text
EOF
}

trunk_exec="trunk"
case "$(uname -s)" in
MINGW* | MSYS* | CYGWIN* | Windows_NT) trunk_exec="trunk.exe" ;;
*) trunk_exec="trunk" ;;
esac

abspath() {
	if [[ $1 == /* ]]; then
		printf "%s\n" "$1"
	else
		printf "%s/%s\n" "$PWD" "$1"
	fi
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

tmpdir() {
	mktemp -d "${TMPDIR:-/tmp}/punchtrunk-setup.XXXXXX"
}

bundle=""
install_dir="/opt/punchtrunk"
env_file=""
checksum_file=""
link_cache=1
force=0

while [[ $# -gt 0 ]]; do
	case "$1" in
	--bundle)
		bundle="$2"
		shift 2
		;;
	--install-dir)
		install_dir="$2"
		shift 2
		;;
	--env-file)
		env_file="$2"
		shift 2
		;;
	--checksum)
		checksum_file="$2"
		shift 2
		;;
	--no-cache-link)
		link_cache=0
		shift
		;;
	--force)
		force=1
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

if [[ -z $bundle ]]; then
	printf "error: --bundle is required\n" >&2
	usage >&2
	exit 1
fi

if [[ ! -f $bundle ]]; then
	printf "error: bundle not found at %s\n" "$bundle" >&2
	exit 1
fi

bundle="$(abspath "$bundle")"
install_dir="$(abspath "$install_dir")"
if [[ -n $env_file ]]; then
	env_file="$(abspath "$env_file")"
else
	env_file="${install_dir}/punchtrunk-airgap.env"
fi

if [[ -n $checksum_file ]]; then
	if [[ ! -f $checksum_file ]]; then
		printf "error: checksum file not found at %s\n" "$checksum_file" >&2
		exit 1
	fi
	checksum_file="$(abspath "$checksum_file")"
elif [[ -f ${bundle}.sha256 ]]; then
	checksum_file="$(abspath "${bundle}.sha256")"
fi

if ! command -v tar >/dev/null 2>&1; then
	printf "error: tar is required to extract the bundle\n" >&2
	exit 1
fi

if [[ -n $checksum_file ]]; then
	expected="$(awk 'NR==1 {print $1}' "$checksum_file")"
	actual="$(compute_sha256 "$bundle")"
	if [[ $expected != "$actual" ]]; then
		printf "error: checksum mismatch for %s\n" "$bundle" >&2
		printf "expected: %s\nactual:   %s\n" "$expected" "$actual" >&2
		exit 1
	fi
fi

mkdir -p "$install_dir"

workdir="$(tmpdir)"
trap 'rm -rf "$workdir"' EXIT

tar -xzf "$bundle" -C "$workdir"

bundle_paths=()
while IFS= read -r dir; do
	bundle_paths+=("$dir")
done < <(find "$workdir" -mindepth 1 -maxdepth 1 -type d)

if [[ ${#bundle_paths[@]} -ne 1 ]]; then
	printf "error: expected bundle to contain a single root directory, found %d\n" "${#bundle_paths[@]}" >&2
	exit 1
fi

bundle_root="${bundle_paths[0]}"
bundle_name="$(basename "$bundle_root")"
target_release="${install_dir}/${bundle_name}"

if [[ -e $target_release ]]; then
	if [[ $force -eq 1 ]]; then
		rm -rf "$target_release"
	else
		printf "error: %s already exists (use --force to overwrite)\n" "$target_release" >&2
		exit 1
	fi
fi

mv "$bundle_root" "$target_release"

ln -sfn "$target_release" "${install_dir}/current"

mkdir -p "${install_dir}/bin"
ln -sfn "${install_dir}/current/bin/punchtrunk" "${install_dir}/bin/punchtrunk"

mkdir -p "${install_dir}/trunk/bin"
ln -sfn "${install_dir}/current/trunk/bin/${trunk_exec}" "${install_dir}/trunk/bin/${trunk_exec}"

copy_directory() {
	local source_dir="$1"
	local target_dir="$2"
	if [[ ! -d $source_dir ]]; then
		return
	fi
	mkdir -p "$target_dir"
	if [[ -z "$(ls -A "$source_dir" 2>/dev/null)" ]]; then
		return
	fi
	tar -C "$source_dir" -cf - . | tar -C "$target_dir" -xf -
}

cache_source="${install_dir}/current/trunk/cache"
cache_target="${install_dir}/cache/trunk"
cache_populated=0
if [[ -d $cache_source ]]; then
	if [[ -d $cache_target && $force -eq 1 ]]; then
		rm -rf "$cache_target"
	fi
	copy_directory "$cache_source" "$cache_target"
	cache_populated=1
fi

if [[ $link_cache -eq 1 ]]; then
	if [[ -z ${HOME-} ]]; then
		printf "warning: HOME is not set, skipping cache symlink\n" >&2
	elif [[ -d $cache_target ]]; then
		cache_home="${HOME}/.cache"
		mkdir -p "$cache_home"
		cache_link="${HOME}/.cache/trunk"
		if [[ -e $cache_link && ! -L $cache_link ]]; then
			if [[ $force -eq 1 ]]; then
				rm -rf "$cache_link"
			else
				printf "warning: %s exists and is not a symlink (use --force to replace)\n" "$cache_link" >&2
				cache_link=""
			fi
		fi
		if [[ -n $cache_link ]]; then
			ln -sfn "$cache_target" "$cache_link"
		fi
	else
		printf "info: bundle did not include a trunk cache; skipping ~/.cache/trunk link\n" >&2
	fi
fi

mkdir -p "$(dirname "$env_file")"
cat >"$env_file" <<EOF
# shellcheck shell=bash
# Source this file to configure PunchTrunk for offline execution.
export PUNCHTRUNK_HOME="${install_dir}/current"
export PUNCHTRUNK_TRUNK_BINARY="${install_dir}/current/trunk/bin/${trunk_exec}"
export PUNCHTRUNK_AIRGAPPED=1
export PATH="${install_dir}/current/bin:${install_dir}/current/trunk/bin:\${PATH}"
EOF

printf "Offline PunchTrunk installed at %s\n" "$install_dir"
printf "Symlinked punchtrunk binary: %s\n" "${install_dir}/bin/punchtrunk"
printf "Environment exports written to %s\n" "$env_file"
if [[ $cache_populated -eq 1 ]]; then
	printf "Cached trunk assets available at %s\n" "$cache_target"
fi
if [[ $link_cache -eq 1 && -d $cache_target && -L ${HOME-}/.cache/trunk ]]; then
	printf "Linked ~/.cache/trunk -> %s\n" "$cache_target"
fi
