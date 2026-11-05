#!/usr/bin/env bash
# Action smoke — runs action.yml's step scripts locally, without going to GitHub.
#
# Why local: a `uses:` line wants a published repo; until that is published, the
# *logic* of action.yml can still be proven. The build step runs here with the
# same command (the action builds from its own source, not an @latest install).
# Proven: the exit contract (0, and 1 with the gate on), GITHUB_OUTPUT
# (flakes/report) in all three formats, the markdown report appended to the step
# summary, the capsule artifact being produced, and witness files being cleaned.
#
# Run: make smoke   (or)   scripts/action-smoke.sh
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/fs-smoke.XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT
BIN="$SCRATCH/flakesmith"
REPO="$SCRATCH/sample-repo"

echo "== 0. binary (from the working tree) =="
(cd "$ROOT" && go build -o "$BIN" ./cmd/flakesmith) && echo "build ok"

echo "== 1. sample repo: green by default, a realistic package that produces pollution =="
mkdir -p "$REPO"
cat > "$REPO/go.mod" <<'EOF'
module example.com/sampleservice

go 1.21
EOF
cat > "$REPO/service_test.go" <<'EOF'
package inventory

import "testing"

var stockCache map[string]int

// Earlier in source order: expects the cache to stay empty — the suite is GREEN as is.
func TestStockReport(t *testing.T) {
	if len(stockCache) != 0 {
		t.Fatalf("cache should have been empty, %d entries present", len(stockCache))
	}
}

// When it runs first it produces pollution: the order polluter.
func TestStockCacheWarm(t *testing.T) {
	if stockCache == nil {
		stockCache = map[string]int{}
	}
	stockCache["sku-1"] = 7
}
EOF
(cd "$REPO" && go test ./... >/dev/null 2>&1) || { echo "UNEXPECTED: sample repo is red"; exit 99; }
echo "default suite green: OK"

export RUNNER_TEMP="$SCRATCH/runner-temp" GITHUB_OUTPUT="$SCRATCH/gh_output" GITHUB_STEP_SUMMARY="$SCRATCH/gh_summary"
export FS_PATHS="./..." FS_BUDGET="200" FS_TIMEOUT="10m" FS_FORMAT="markdown" FS_FAIL="false"
mkdir -p "$RUNNER_TEMP"; : > "$GITHUB_OUTPUT"; : > "$GITHUB_STEP_SUMMARY"
cd "$REPO"

# action.yml "hunt flakes" step script — the flakesmith command is the local binary.
run_hunt_step() {
  report="$RUNNER_TEMP/flakesmith-report.$FS_FORMAT"
  args=(hunt $FS_PATHS --format "$FS_FORMAT" --budget "$FS_BUDGET" --timeout "$FS_TIMEOUT")
  [ "$FS_FAIL" = "true" ] && args+=(--fail-on-flake)

  "$BIN" "${args[@]}" | tee "$report"
  status=${PIPESTATUS[0]}

  case "$FS_FORMAT" in
    json) count=$(grep -c '"id":' "$report" || true) ;;
    *)    count=$(grep -Ec '^(### flake:|flake found:)' "$report" || true) ;;
  esac
  echo "flakes=${count:-0}" >> "$GITHUB_OUTPUT"
  echo "report=$report" >> "$GITHUB_OUTPUT"
  return "$status"
}

echo "== 2-6. step scripts + checks =="
run_hunt_step; status=$?
report="$RUNNER_TEMP/flakesmith-report.$FS_FORMAT"
[ -f "$report" ] && cat "$report" >> "$GITHUB_STEP_SUMMARY"
# @actions/glob's `**/.flakesmith/*.json` pattern also matches zero segments;
# the local equivalent uses find (no globstar in bash 3.2).
capsule_count=$(find . -name '*.json' -path '*.flakesmith*' | grep -c . || true)

fail=0
[ "$status" = "0" ] || { echo "FAIL: expected exit 0 with the gate off, got $status"; fail=1; }
grep -q "flakes=1" "$GITHUB_OUTPUT" || { echo "FAIL: GITHUB_OUTPUT flakes=1: $(cat "$GITHUB_OUTPUT")"; fail=1; }
grep -q "^### flake: " "$report" || { echo "FAIL: no markdown heading"; fail=1; }
[ -s "$GITHUB_STEP_SUMMARY" ] || { echo "FAIL: step summary empty"; fail=1; }
[ "$capsule_count" -ge 1 ] || { echo "FAIL: no capsule artifact"; fail=1; }
[ -z "$(find . -name 'zz_flakesmith_witness_*')" ] || { echo "FAIL: witness leftovers"; fail=1; }

# json format: the count must still be 1 and the output must be a single valid document.
FS_FORMAT=json; : > "$GITHUB_OUTPUT"; run_hunt_step >/dev/null; json_status=$?
grep -q "flakes=1" "$GITHUB_OUTPUT" || { echo "FAIL: not flakes=1 in json format: $(cat "$GITHUB_OUTPUT")"; fail=1; }
python3 - "$RUNNER_TEMP/flakesmith-report.json" <<'PY' || { echo "FAIL: json output is not a single document"; fail=1; }
import json,sys
json.load(open(sys.argv[1]))
PY
[ "$json_status" = "0" ] || { echo "FAIL: json run exit $json_status"; fail=1; }
FS_FORMAT=markdown

FS_FAIL=true; : > "$GITHUB_OUTPUT"; run_hunt_step >/dev/null; gated=$?
[ "$gated" = "1" ] || { echo "FAIL: expected exit 1 with the gate on, got $gated"; fail=1; }

if [ "$fail" = "0" ]; then echo "SMOKE: PASSED ✅"; else echo "SMOKE: FAILED ❌"; exit 1; fi
