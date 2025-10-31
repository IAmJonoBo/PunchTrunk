#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
  export REPO_ROOT
}

build_punchtrunk() {
  local output_bin="$1"
  if ! command -v go >/dev/null 2>&1; then
    skip "go toolchain not available"
  fi
  run env CGO_ENABLED=0 go build -o "${output_bin}" ./cmd/punchtrunk
  [ "$status" -eq 0 ] || {
    echo "go build failed" >&2
    echo "$output" >&2
    return 1
  }
}

create_trunk_fixture() {
  local target_file="$1"
  local payload="$2"
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  printf '%s' "$payload" >"${tmp_dir}/trunk"
  tar -C "${tmp_dir}" -czf "${target_file}" trunk
  rm -rf "${tmp_dir}"
}

create_curl_stub() {
  local stub_path="$1"
  cat >"${stub_path}" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
url=""
for arg in "$@"; do
  if [[ "$arg" == -* ]]; then
    continue
  fi
  url="$arg"
done
if [[ -z "${url}" ]]; then
  echo "curl stub missing URL" >&2
  exit 1
fi
if [[ "$url" == *"linux"* ]]; then
  cat "${TRUNK_FIXTURE_LINUX}"
elif [[ "$url" == *"darwin"* ]]; then
  cat "${TRUNK_FIXTURE_DARWIN}"
else
  echo "curl stub has no fixture for $url" >&2
  exit 1
fi
STUB
  chmod +x "${stub_path}"
}

@test "auto-downloads trunk binary when default path missing for linux" {
  punch_bin="${BATS_TMPDIR}/bin/punchtrunk"
  mkdir -p "$(dirname "${punch_bin}")"
  build_punchtrunk "${punch_bin}"

  linux_fixture="${BATS_TMPDIR}/trunk-linux.tar.gz"
  create_trunk_fixture "${linux_fixture}" "linux stub trunk"

  mock_bin="${BATS_TMPDIR}/mock-bin-linux"
  mkdir -p "${mock_bin}"
  create_curl_stub "${mock_bin}/curl"

  export PATH="${mock_bin}:${PATH}"
  export TRUNK_FIXTURE_LINUX="${linux_fixture}"
  export TRUNK_FIXTURE_DARWIN="${linux_fixture}"
  export HOME="${BATS_TMPDIR}/home-linux"
  mkdir -p "${HOME}"

  dist_dir="${BATS_TMPDIR}/dist-linux"
  mkdir -p "${dist_dir}"
  bundle_name="test-linux-bundle.tar.gz"

  run "${REPO_ROOT}/scripts/build-offline-bundle.sh" \
    --punchtrunk-binary "${punch_bin}" \
    --config-dir "${REPO_ROOT}/.trunk" \
    --output-dir "${dist_dir}" \
    --bundle-name "${bundle_name}" \
    --target-os linux \
    --target-arch amd64 \
    --skip-hydrate \
    --no-cache \
    --force

  [ "$status" -eq 0 ] || { echo "$output"; return 1; }
  [[ "$output" == *"Auto-downloaded trunk binary for linux/amd64"* ]]
  [[ "$output" == *"Bundle created"* ]]

  bundle_path="${dist_dir}/${bundle_name}"
  [ -f "${bundle_path}" ]

  bundle_root="${bundle_name%.tar.gz}"
  run tar -xOf "${bundle_path}" "${bundle_root}/trunk/bin/trunk"
  [ "$status" -eq 0 ]
  [[ "$output" == "linux stub trunk" ]]

  run tar -xOf "${bundle_path}" "${bundle_root}/manifest.json"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"trunk_binary": "trunk"'* ]]
}

@test "auto-downloads trunk binary when default path missing for macos" {
  punch_bin="${BATS_TMPDIR}/bin/punchtrunk-mac"
  mkdir -p "$(dirname "${punch_bin}")"
  build_punchtrunk "${punch_bin}"

  darwin_fixture="${BATS_TMPDIR}/trunk-darwin.tar.gz"
  create_trunk_fixture "${darwin_fixture}" "darwin stub trunk"

  mock_bin="${BATS_TMPDIR}/mock-bin-darwin"
  mkdir -p "${mock_bin}"
  create_curl_stub "${mock_bin}/curl"

  export PATH="${mock_bin}:${PATH}"
  export TRUNK_FIXTURE_LINUX="${darwin_fixture}"
  export TRUNK_FIXTURE_DARWIN="${darwin_fixture}"
  export HOME="${BATS_TMPDIR}/home-darwin"
  mkdir -p "${HOME}"

  dist_dir="${BATS_TMPDIR}/dist-darwin"
  mkdir -p "${dist_dir}"
  bundle_name="test-darwin-bundle.tar.gz"

  run "${REPO_ROOT}/scripts/build-offline-bundle.sh" \
    --punchtrunk-binary "${punch_bin}" \
    --config-dir "${REPO_ROOT}/.trunk" \
    --output-dir "${dist_dir}" \
    --bundle-name "${bundle_name}" \
    --target-os darwin \
    --target-arch arm64 \
    --skip-hydrate \
    --no-cache \
    --force

  [ "$status" -eq 0 ] || { echo "$output"; return 1; }
  [[ "$output" == *"Auto-downloaded trunk binary for darwin/arm64"* ]]

  bundle_path="${dist_dir}/${bundle_name}"
  [ -f "${bundle_path}" ]

  bundle_root="${bundle_name%.tar.gz}"
  run tar -xOf "${bundle_path}" "${bundle_root}/trunk/bin/trunk"
  [ "$status" -eq 0 ]
  [[ "$output" == "darwin stub trunk" ]]
}

@test "default bundle name honours normalized target overrides" {
  punch_bin="${BATS_TMPDIR}/bin/punchtrunk-target"
  mkdir -p "$(dirname "${punch_bin}")"
  build_punchtrunk "${punch_bin}"

  trunk_stub="${BATS_TMPDIR}/trunk-win.exe"
  cat >"${trunk_stub}" <<'EOF'
#!/usr/bin/env bash
echo "stub trunk"
EOF
  chmod +x "${trunk_stub}"

  dist_dir="${BATS_TMPDIR}/dist-target"
  mkdir -p "${dist_dir}"

  run "${REPO_ROOT}/scripts/build-offline-bundle.sh" \
    --punchtrunk-binary "${punch_bin}" \
    --config-dir "${REPO_ROOT}/.trunk" \
    --output-dir "${dist_dir}" \
    --trunk-binary "${trunk_stub}" \
    --target-os WINDOWS \
    --target-arch AARCH64 \
    --skip-hydrate \
    --no-cache \
    --force

  [ "$status" -eq 0 ] || { echo "$output"; return 1; }
  expected_bundle="punchtrunk-offline-windows-arm64.tar.gz"
  [[ "$output" == *"Bundle created: ${dist_dir}/${expected_bundle}"* ]]

  bundle_path="${dist_dir}/${expected_bundle}"
  [ -f "${bundle_path}" ]

<<<<<<< HEAD
  run bash -c "tar -tf \"${bundle_path}\" | grep 'trunk/bin/trunk.exe'"
=======
  run tar -tf "${bundle_path}"
>>>>>>> 9b5cdee (feat: add Semgrep integration and update build scripts; enhance QA documentation)
  [ "$status" -eq 0 ]
  [[ "$output" == *"trunk/bin/trunk.exe"* ]]
}
