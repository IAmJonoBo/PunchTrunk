#!/usr/bin/env bash
# PunchTrunk Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash

set -euo pipefail

# Configuration
REPO="IAmJonoBo/PunchTrunk"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="punchtrunk"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
	echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
	echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
	echo -e "${RED}[ERROR]${NC} $1"
	exit 1
}

# Detect OS and architecture
detect_platform() {
	local os
	local arch

	local uname_s
	if ! uname_s=$(uname -s); then
		error "Failed to detect operating system"
	fi

	case "${uname_s}" in
	Linux*) os="linux" ;;
	Darwin*) os="darwin" ;;
	MINGW* | MSYS* | CYGWIN*) os="windows" ;;
	*) error "Unsupported operating system: ${uname_s}" ;;
	esac

	local uname_m
	if ! uname_m=$(uname -m); then
		error "Failed to detect architecture"
	fi

	case "${uname_m}" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*) error "Unsupported architecture: ${uname_m}" ;;
	esac

	echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
	local version
	version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

	if [[ -z ${version} ]]; then
		error "Failed to fetch latest version"
	fi

	echo "${version}"
}

# Download and install binary
install_binary() {
	local platform="$1"
	local version="$2"
	local binary_name="${BINARY_NAME}"
	local ext=""

	if [[ ${platform} == *"windows"* ]]; then
		binary_name="${BINARY_NAME}.exe"
		ext=".exe"
	fi

	local download_name="${BINARY_NAME}-${platform}${ext}"
	local download_url="https://github.com/${REPO}/releases/download/${version}/${download_name}"
	local checksum_url="${download_url}.sha256"
	local tmp_dir
	tmp_dir=$(mktemp -d)

	info "Downloading PunchTrunk ${version} for ${platform}..."

	# Download binary
	if ! curl -fsSL "${download_url}" -o "${tmp_dir}/${binary_name}"; then
		error "Failed to download binary from ${download_url}"
	fi

	# Download checksum
	if ! curl -fsSL "${checksum_url}" -o "${tmp_dir}/${binary_name}.sha256"; then
		warn "Failed to download checksum file, skipping verification"
	else
		info "Verifying checksum..."
		if command -v sha256sum >/dev/null 2>&1; then
			if (
				cd "${tmp_dir}" || exit 1
				sha256sum -c "${binary_name}.sha256"
			); then
				:
			else
				error "Checksum verification failed"
			fi
		elif command -v shasum >/dev/null 2>&1; then
			if (
				cd "${tmp_dir}" || exit 1
				shasum -a 256 -c "${binary_name}.sha256"
			); then
				:
			else
				error "Checksum verification failed"
			fi
		else
			warn "sha256sum not available, skipping checksum verification"
		fi
	fi

	# Make executable
	chmod +x "${tmp_dir}/${binary_name}"

	# Install to target directory
	info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."

	if [[ -w ${INSTALL_DIR} ]]; then
		mv "${tmp_dir}/${binary_name}" "${INSTALL_DIR}/${BINARY_NAME}"
	else
		sudo mv "${tmp_dir}/${binary_name}" "${INSTALL_DIR}/${BINARY_NAME}"
	fi

	# Cleanup
	rm -rf "${tmp_dir}"

	info "✓ PunchTrunk ${version} installed successfully!"
}

# Verify installation
verify_installation() {
	if ! command -v "${BINARY_NAME}" >/dev/null 2>&1; then
		warn "Binary installed but not in PATH. Add ${INSTALL_DIR} to your PATH:"
		echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
		return 1
	fi

	info "Verifying installation..."
	if ! "${BINARY_NAME}" --help >/dev/null 2>&1; then
		error "Installation verification failed"
	fi

	local installed_version
	installed_version=$("${BINARY_NAME}" --help 2>&1 | head -1 || echo "unknown")
	info "✓ Installation verified (${installed_version})"
}

# Main installation flow
main() {
	echo "================================================"
	echo "  PunchTrunk Installation Script"
	echo "================================================"
	echo ""

	# Detect platform
	local platform
	platform=$(detect_platform)
	info "Detected platform: ${platform}"

	# Get version
	local version="${VERSION-}"
	if [[ -z ${version} ]]; then
		info "Fetching latest version..."
		version=$(get_latest_version)
	fi
	info "Installing version: ${version}"

	# Check if already installed
	if command -v "${BINARY_NAME}" >/dev/null 2>&1; then
		local current_version
		current_version=$("${BINARY_NAME}" --help 2>&1 | head -1 || echo "unknown")
		warn "PunchTrunk is already installed: ${current_version}"
		read -p "Do you want to reinstall? (y/N) " -n 1 -r
		echo
		if [[ ! ${REPLY} =~ ^[Yy]$ ]]; then
			info "Installation cancelled"
			exit 0
		fi
	fi

	# Install
	install_binary "${platform}" "${version}"

	# Verify
	verify_installation

	echo ""
	echo "================================================"
	echo "  Installation Complete!"
	echo "================================================"
	echo ""
	echo "Next steps:"
	echo ""
	echo "  1. Initialize Trunk in your repository:"
	echo "     $ cd your-repo"
	echo "     $ trunk init"
	echo ""
	echo "  2. Run PunchTrunk:"
	echo "     $ punchtrunk --mode fmt,lint,hotspots"
	echo ""
	echo "  3. View hotspots:"
	echo "     $ cat reports/hotspots.sarif | jq '.runs[0].results'"
	echo ""
	echo "Documentation:"
	echo "  https://github.com/${REPO}/blob/main/README.md"
	echo ""
}

main "$@"
