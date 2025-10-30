#!/usr/bin/env bash
# validate-agent-environment.sh
# 
# Comprehensive validation script for agent/runner environments.
# Checks that all required tools for PunchTrunk are available and properly configured.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
CHECKS_WARNED=0
TOTAL_CHECKS=0

log_info() {
	printf "${BLUE}[INFO]${NC} %s\n" "$1"
}

log_success() {
	printf "${GREEN}[✓]${NC} %s\n" "$1"
	((CHECKS_PASSED++))
	((TOTAL_CHECKS++))
}

log_fail() {
	printf "${RED}[✗]${NC} %s\n" "$1"
	((CHECKS_FAILED++))
	((TOTAL_CHECKS++))
}

log_warn() {
	printf "${YELLOW}[⚠]${NC} %s\n" "$1"
	((CHECKS_WARNED++))
}

check_command() {
	local cmd="$1"
	local name="${2:-$cmd}"
	local required="${3:-true}"
	
	if command -v "$cmd" >/dev/null 2>&1; then
		local version=""
		case "$cmd" in
			git)
				version=$(git --version 2>/dev/null | head -1)
				;;
			trunk)
				version=$(trunk --version 2>/dev/null | head -1)
				;;
			go)
				version=$(go version 2>/dev/null | head -1)
				;;
			python|python3)
				version=$(python3 --version 2>/dev/null || python --version 2>/dev/null)
				;;
			pip|pip3)
				version=$(pip3 --version 2>/dev/null || pip --version 2>/dev/null)
				;;
			node)
				version=$(node --version 2>/dev/null)
				;;
			npm)
				version=$(npm --version 2>/dev/null)
				;;
			jq)
				version=$(jq --version 2>/dev/null)
				;;
			curl)
				version=$(curl --version 2>/dev/null | head -1)
				;;
			bash)
				version="$BASH_VERSION"
				;;
			*)
				version="available"
				;;
		esac
		log_success "$name is available${version:+: $version}"
		return 0
	else
		if [ "$required" = "true" ]; then
			log_fail "$name is NOT available (required)"
			return 1
		else
			log_warn "$name is NOT available (optional)"
			return 0
		fi
	fi
}

check_file() {
	local file="$1"
	local name="$2"
	local required="${3:-true}"
	
	if [ -f "$file" ]; then
		log_success "$name exists at $file"
		return 0
	else
		if [ "$required" = "true" ]; then
			log_fail "$name does NOT exist at $file (required)"
			return 1
		else
			log_warn "$name does NOT exist at $file (optional)"
			return 0
		fi
	fi
}

check_directory() {
	local dir="$1"
	local name="$2"
	local writable="${3:-false}"
	
	if [ -d "$dir" ]; then
		if [ "$writable" = "true" ]; then
			if [ -w "$dir" ]; then
				log_success "$name exists and is writable at $dir"
			else
				log_fail "$name exists but is NOT writable at $dir"
				return 1
			fi
		else
			log_success "$name exists at $dir"
		fi
		return 0
	else
		log_warn "$name does NOT exist at $dir (will be created if needed)"
		return 0
	fi
}

check_executable() {
	local file="$1"
	local name="$2"
	
	if [ -x "$file" ]; then
		local version=""
		if [[ "$file" == *"punchtrunk"* ]]; then
			version=$("$file" --version 2>/dev/null || echo "version unknown")
		fi
		log_success "$name is executable at $file${version:+ ($version)}"
		return 0
	elif [ -f "$file" ]; then
		log_fail "$name exists but is NOT executable at $file"
		return 1
	else
		log_fail "$name does NOT exist at $file"
		return 1
	fi
}

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  PunchTrunk Agent Environment Validation"
echo "═══════════════════════════════════════════════════════"
echo ""

# ============================================================
# Section 1: Core System Tools
# ============================================================
log_info "Checking core system tools..."
echo ""

check_command git "Git" true
check_command bash "Bash" true
check_command curl "curl" false

echo ""

# ============================================================
# Section 2: Trunk CLI
# ============================================================
log_info "Checking Trunk CLI..."
echo ""

if check_command trunk "Trunk CLI" false; then
	# Check if trunk is in PATH
	TRUNK_PATH=$(command -v trunk)
	log_info "Trunk CLI location: $TRUNK_PATH"
	
	# Try to get trunk version
	if TRUNK_VERSION=$(trunk --version 2>/dev/null); then
		log_info "Trunk CLI version: $TRUNK_VERSION"
	fi
else
	# Check if trunk exists in standard locations
	if [ -x "$HOME/.trunk/bin/trunk" ]; then
		log_warn "Trunk CLI found at $HOME/.trunk/bin/trunk but not in PATH"
		log_info "Add to PATH: export PATH=\"\$HOME/.trunk/bin:\$PATH\""
	else
		log_warn "Trunk CLI not found. PunchTrunk can auto-install it, or install manually:"
		log_info "  curl https://get.trunk.io -fsSL | bash -s -- -y"
	fi
fi

echo ""

# ============================================================
# Section 3: Go Toolchain
# ============================================================
log_info "Checking Go toolchain..."
echo ""

if check_command go "Go" false; then
	GO_VERSION=$(go version 2>/dev/null | awk '{print $3}')
	if [[ "$GO_VERSION" == go1.2* ]] || [[ "$GO_VERSION" > "go1.22" ]]; then
		log_info "Go version is compatible (>= 1.22): $GO_VERSION"
	else
		log_warn "Go version may be too old: $GO_VERSION (recommended: >= 1.22)"
	fi
fi

echo ""

# ============================================================
# Section 4: Python & Semgrep (Optional)
# ============================================================
log_info "Checking Python & Semgrep (optional for security scanning)..."
echo ""

check_command python3 "Python" false
check_command pip3 "pip" false

if command -v pip3 >/dev/null 2>&1 || command -v pip >/dev/null 2>&1; then
	if command -v semgrep >/dev/null 2>&1; then
		check_command semgrep "Semgrep" false
	else
		log_warn "Semgrep not installed. Install with: pip install semgrep"
	fi
fi

echo ""

# ============================================================
# Section 5: PunchTrunk Binary
# ============================================================
log_info "Checking PunchTrunk binary..."
echo ""

cd "$ROOT_DIR" || exit 1

if [ -x "bin/punchtrunk" ]; then
	check_executable "bin/punchtrunk" "PunchTrunk binary"
else
	log_warn "PunchTrunk binary not found at bin/punchtrunk"
	log_info "Build with: make build"
fi

echo ""

# ============================================================
# Section 6: Configuration Files
# ============================================================
log_info "Checking configuration files..."
echo ""

check_file ".trunk/trunk.yaml" "Trunk configuration" true
check_file "Makefile" "Makefile" true
check_file "go.mod" "Go module file" false

echo ""

# ============================================================
# Section 7: Directory Structure
# ============================================================
log_info "Checking directory structure..."
echo ""

check_directory ".trunk" "Trunk config directory" false
check_directory "$HOME/.cache/trunk" "Trunk cache directory" true
check_directory "scripts" "Scripts directory" false
check_directory "cmd/punchtrunk" "PunchTrunk source directory" false

echo ""

# ============================================================
# Section 8: Git Repository
# ============================================================
log_info "Checking git repository..."
echo ""

if [ -d ".git" ]; then
	log_success "Git repository initialized"
	
	# Check git config
	if git config user.name >/dev/null 2>&1 && git config user.email >/dev/null 2>&1; then
		log_success "Git user configured"
	else
		log_warn "Git user not configured. Some tests may fail."
		log_info "Configure with: git config --global user.name 'Your Name'"
		log_info "               git config --global user.email 'your@email.com'"
	fi
	
	# Check for commits
	if git rev-parse HEAD >/dev/null 2>&1; then
		log_success "Git repository has commits"
	else
		log_warn "Git repository has no commits yet"
	fi
else
	log_fail "Not a git repository"
fi

echo ""

# ============================================================
# Section 9: Environment Variables
# ============================================================
log_info "Checking environment variables..."
echo ""

if [ -n "${PUNCHTRUNK_AIRGAPPED:-}" ]; then
	log_info "PUNCHTRUNK_AIRGAPPED is set: $PUNCHTRUNK_AIRGAPPED"
	if [ -n "${PUNCHTRUNK_TRUNK_BINARY:-}" ]; then
		log_info "PUNCHTRUNK_TRUNK_BINARY is set: $PUNCHTRUNK_TRUNK_BINARY"
	else
		log_warn "PUNCHTRUNK_AIRGAPPED is set but PUNCHTRUNK_TRUNK_BINARY is not"
	fi
else
	log_info "PUNCHTRUNK_AIRGAPPED not set (network access expected)"
fi

if [ -n "${TRUNK_CACHE_DIR:-}" ]; then
	log_info "TRUNK_CACHE_DIR is set: $TRUNK_CACHE_DIR"
fi

if [ -n "${GITHUB_ACTIONS:-}" ]; then
	log_info "Running in GitHub Actions environment"
fi

echo ""

# ============================================================
# Section 10: Diagnostic Mode Tests
# ============================================================
log_info "Running diagnostic tests..."
echo ""

if [ -x "bin/punchtrunk" ]; then
	# Test diagnose-airgap mode
	log_info "Running: bin/punchtrunk --mode diagnose-airgap"
	if ./bin/punchtrunk --mode diagnose-airgap >/dev/null 2>&1; then
		log_success "diagnose-airgap mode passed"
	else
		log_warn "diagnose-airgap mode reported issues (check details above)"
	fi
else
	log_warn "Skipping diagnostic tests (punchtrunk binary not available)"
fi

echo ""

# ============================================================
# Summary
# ============================================================
echo "═══════════════════════════════════════════════════════"
echo "  Validation Summary"
echo "═══════════════════════════════════════════════════════"
echo ""
printf "Total checks: %d\n" "$TOTAL_CHECKS"
printf "${GREEN}Passed: %d${NC}\n" "$CHECKS_PASSED"
printf "${RED}Failed: %d${NC}\n" "$CHECKS_FAILED"
printf "${YELLOW}Warnings: %d${NC}\n" "$CHECKS_WARNED"
echo ""

if [ "$CHECKS_FAILED" -eq 0 ]; then
	echo "${GREEN}✅ Environment validation PASSED${NC}"
	echo ""
	log_info "Your environment is ready to run PunchTrunk"
	log_info "Next steps:"
	echo "  - Build PunchTrunk: make build"
	echo "  - Run tests: make test"
	echo "  - Run PunchTrunk: ./bin/punchtrunk --mode fmt,lint,hotspots"
	echo ""
	exit 0
else
	echo "${RED}❌ Environment validation FAILED${NC}"
	echo ""
	log_info "Please address the failed checks above"
	log_info "See: docs/AGENT_PROVISIONING.md for detailed setup instructions"
	echo ""
	exit 1
fi
