#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
  export REPO_ROOT
  export HOME="${BATS_TMPDIR}/home"
  mkdir -p "${HOME}"
}

@test "requires bundle argument" {
  run "${REPO_ROOT}/scripts/setup-airgap.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"--bundle is required"* ]]
}

@test "setup script passes shellcheck" {
  if ! command -v shellcheck >/dev/null 2>&1; then
    skip "shellcheck not installed"
  fi
  run shellcheck "${REPO_ROOT}/scripts/setup-airgap.sh"
  [ "$status" -eq 0 ]
}

@test "installs offline bundle and wires cache" {
  if ! command -v go >/dev/null 2>&1; then
    skip "go toolchain not available"
  fi
  if ! command -v tar >/dev/null 2>&1; then
    skip "tar not available"
  fi

  cd "${REPO_ROOT}"

  punch_bin="${BATS_TMPDIR}/bin/punchtrunk"
  mkdir -p "$(dirname "${punch_bin}")"
  run env CGO_ENABLED=0 go build -o "${punch_bin}" ./cmd/punchtrunk
  [ "$status" -eq 0 ] || { echo "go build failed"; echo "$output"; return 1; }

  trunk_dir="${BATS_TMPDIR}/trunk"
  mkdir -p "${trunk_dir}"
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*|Windows_NT)
      trunk_stub="${trunk_dir}/trunk.exe"
      ;;
    *)
      trunk_stub="${trunk_dir}/trunk"
      ;;
  esac
  cat >"${trunk_stub}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--version" ]]; then
  echo "stub trunk version 0.0.0"
  exit 0
fi
exit 0
EOF
  chmod +x "${trunk_stub}"

  cache_dir="${BATS_TMPDIR}/cache"
  mkdir -p "${cache_dir}"
  echo "demo" > "${cache_dir}/tool.lock"

  dist_dir="${BATS_TMPDIR}/dist"
  mkdir -p "${dist_dir}"
  bundle_name="test-offline-bundle.tgz"
  run "${REPO_ROOT}/scripts/build-offline-bundle.sh" \
    --punchtrunk-binary "${punch_bin}" \
    --trunk-binary "${trunk_stub}" \
    --cache-dir "${cache_dir}" \
    --config-dir "${REPO_ROOT}/.trunk" \
    --output-dir "${dist_dir}" \
    --bundle-name "${bundle_name}" \
    --force
  [ "$status" -eq 0 ] || { echo "$output"; return 1; }

  bundle_path="${dist_dir}/${bundle_name}"
  [ -f "${bundle_path}" ]

  install_dir="${BATS_TMPDIR}/install"
  run "${REPO_ROOT}/scripts/setup-airgap.sh" \
    --bundle "${bundle_path}" \
    --install-dir "${install_dir}" \
    --force
  [ "$status" -eq 0 ] || { echo "$output"; return 1; }

  [ -L "${install_dir}/current" ]
  [ -L "${install_dir}/bin/punchtrunk" ]
  [ -L "${install_dir}/trunk/bin/$(basename "${trunk_stub}")" ]
  [ -d "${install_dir}/cache/trunk" ]

  env_file="${install_dir}/punchtrunk-airgap.env"
  [ -f "${env_file}" ]
  grep -q "PUNCHTRUNK_HOME" "${env_file}"
  grep -q "PUNCHTRUNK_TRUNK_BINARY" "${env_file}"
  grep -q "PUNCHTRUNK_AIRGAPPED" "${env_file}"

  if [[ -d "${install_dir}/cache/trunk" ]]; then
    [ -L "${HOME}/.cache/trunk" ]
    readlink "${HOME}/.cache/trunk" | grep -q "${install_dir}/cache/trunk"
  fi
}
