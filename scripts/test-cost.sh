#!/usr/bin/env bash
# test-cost.sh — what the test suite costs, and where.
#
# Why this exists: the suite's wall time is dominated by integration tests that
# spawn real `go test` subprocesses, so "CI got slower" is almost never an
# assertion getting heavier — it is one more subprocess-driving test, or one
# more repeat of a package that spawns. This prints the numbers that answer
# that question without reading a CI log: wall time, per-package time, and the
# slowest tests.
#
# Not a gate. A wall-time assertion is exactly the flaky thing this repo spent
# issue #12 removing. It is an operator tool: run it before and after a
# test-layout change and compare.
#
# On a subprocess COUNTER: a PATH shim cannot see the suite's `go test` calls.
# The go command prepends its own GOROOT/bin to PATH for the processes it
# starts, so a child resolves the real toolchain before any shim. Measured, not
# assumed: the shim reported one call while a single hunt spawns 33. Left out
# rather than printing a number that means something else.
#
# Usage:
#   scripts/test-cost.sh                      # whole suite
#   scripts/test-cost.sh ./internal/cli/...   # one subtree
set -uo pipefail
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 1

pkgs="${1:-./...}"
out=$(mktemp)
timing=$(mktemp)
trap 'rm -f "$out" "$timing"' EXIT

echo "== suite ($pkgs)"
# shellcheck disable=SC2086
/usr/bin/time -p go test -count=1 -v $pkgs >"$out" 2>"$timing" || true
grep -E '^(real|user|sys)' "$timing" | sed 's/^/  /'
failed=$(grep -c '^--- FAIL' "$out")
[ "$failed" -gt 0 ] && echo "  FAILED tests: $failed"

echo
echo "== per package (slowest first)"
grep -E '^ok  ' "$out" | sed -E 's/^ok[[:space:]]+([^[:space:]]+)[[:space:]]+([0-9.]+)s.*/\2 \1/' | sort -rn

echo
echo "== slowest tests"
grep -E '^ *(--- (PASS|FAIL): )' "$out" |
  sed -E 's/^ *--- (PASS|FAIL): ([^ ]+) \(([0-9.]+)s\)/\3 \2/' |
  sort -rn | head -10
