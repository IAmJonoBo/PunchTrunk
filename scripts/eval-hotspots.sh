#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BIN="$ROOT_DIR/bin/punchtrunk"
BASELINE="$ROOT_DIR/testdata/sarif/hotspots-fixture.sarif"
TRUNK_CONFIG="$ROOT_DIR/.trunk"

if [[ ! -x $BIN ]]; then
	echo "error: PunchTrunk binary not found at $BIN. Run 'make build' first." >&2
	exit 1
fi

WORK_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t punchtrunk-eval)
cleanup() {
	rm -rf "$WORK_DIR"
}
trap cleanup EXIT

pushd "$WORK_DIR" >/dev/null

git init -q

git config user.email "ci@example.com"
git config user.name "PunchTrunk CI"

cat >main.go <<'EOF'
package main

func hello() string { return "hi" }
EOF

git add main.go
git commit -q -m "initial commit"

cat >main.go <<'EOF'
package main

func hello() string {
	return "hi there"
}

func helper() int { return 42 }
EOF

cat >utils.go <<'EOF'
package main

func repeat(input string, times int) string {
	result := ""
	for i := 0; i < times; i++ {
		result += input
	}
	return result
}
EOF

git add main.go utils.go
git commit -q -m "add helper and utils"

"$BIN" \
	--mode hotspots \
	--base-branch HEAD~1 \
	--trunk-config-dir "$TRUNK_CONFIG"

OUTPUT="$WORK_DIR/reports/hotspots.sarif"

if [[ ! -f $OUTPUT ]]; then
	echo "error: expected SARIF at $OUTPUT" >&2
	exit 1
fi

jq . "$OUTPUT" >/dev/null

if command -v sarif >/dev/null 2>&1; then
	sarif validate "$OUTPUT"
else
	echo "warning: 'sarif' CLI not found; skipping sarif-tools validation" >&2
fi

if [[ ! -f $BASELINE ]]; then
	if [[ ${PUNCHTRUNK_CREATE_BASELINE:-0} == "1" ]]; then
		mkdir -p "$(dirname "$BASELINE")"
		cp "$OUTPUT" "$BASELINE"
		echo "Baseline SARIF written to $BASELINE"
		exit 0
	fi

	echo "error: baseline SARIF missing at $BASELINE" >&2
	exit 1
fi

if ! cmp -s "$BASELINE" "$OUTPUT"; then
	echo "error: hotspots output diverged from baseline" >&2
	diff -u "$BASELINE" "$OUTPUT" || true
	exit 1
fi

echo "Hotspot evaluation succeeded"

popd >/dev/null
