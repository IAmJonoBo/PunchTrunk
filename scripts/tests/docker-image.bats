#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
  export REPO_ROOT
}

@test "Docker image bundles Trunk CLI and works air-gapped" {
  if ! command -v docker >/dev/null 2>&1; then
    skip "docker not installed"
  fi
  cd "$REPO_ROOT"
  image_tag="punchtrunk-test:airgap-$(date +%s)"
  run docker build -t "$image_tag" .
  [ "$status" -eq 0 ] || { echo "docker build failed"; echo "$output"; return 1; }

  # Run container with air-gapped env
  run docker run --rm -e PUNCHTRUNK_AIRGAPPED=1 "$image_tag" /app/trunk --version
  [ "$status" -eq 0 ]
  [[ "$output" == *"trunk version"* ]]

  # Check PUNCHTRUNK_TRUNK_BINARY is set and executable
  run docker run --rm -e PUNCHTRUNK_AIRGAPPED=1 "$image_tag" sh -c '[ -x "$PUNCHTRUNK_TRUNK_BINARY" ] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == "OK" ]]

  # Optionally, check punchtrunk runs (help output)
  run docker run --rm -e PUNCHTRUNK_AIRGAPPED=1 "$image_tag" /app/punchtrunk --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"PunchTrunk"* ]]
}
