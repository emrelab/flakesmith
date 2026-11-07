#!/usr/bin/env bash
# Autonomous-run gate (docs/AUTO.md §5).
#
# Stricter than `scripts/pre-commit`, which only covers the packages a commit
# touches and must stay fast for humans. This one is the gate a run must pass
# before its commit: repo-wide build/vet, the full race suite, the action smoke
# script, every fixture module, plus three mechanical guards for the quiet
# corruptions (weakened tests, a widened dependency surface, a CI edit).
#
# Exit 0 = green. Any failure exits 1 with the failing command printed.
#
# Deliberately NOT cached: -count=1 everywhere. A cached pass is not a pass.

set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 1

# Tier decides one thing only: whether the diff may touch CI. `auto:safe` is
# auto-merged by the run, so a CI edit there would let the run weaken its own
# gate and merge the weakening — absolutely banned. `auto:review` is merged by a
# human, who reads the diff, so the edit is allowed and must be called out in
# the PR body (docs/AUTO.md §7).
TIER=safe
if [ "${1:-}" = "--tier" ]; then TIER=${2:-safe}; fi
case "$TIER" in safe|review) ;; *) echo "usage: verify.sh [--tier safe|review]" >&2; exit 2 ;; esac

red=$'\033[31m'; green=$'\033[32m'; off=$'\033[0m'
fail() {
  printf '%sGATE RED%s  %s\n' "$red" "$off" "$1" >&2
  printf 'Fix the cause. Do not weaken the gate, a test, or CI — see docs/AUTO.md §5.\n' >&2
  exit 1
}
step() { printf '\n== %s\n' "$1"; }

base=$(git rev-parse --verify --quiet origin/main >/dev/null 2>&1 && echo origin/main || echo HEAD)

# ---------------------------------------------------------------- 1. guards
# Cosmetic, mechanical, and each one is a way an unattended run corrupts a repo
# without a single red test. Run before the expensive steps.

step "guard: CI is human-owned"
if ! git diff --quiet "$base" -- .github/workflows action.yml; then
  if [ "$TIER" = "safe" ]; then
    git diff --stat "$base" -- .github/workflows action.yml >&2
    fail "auto:safe may not touch .github/ or action.yml — a run must never merge its own CI edit"
  fi
  printf 'allowed at tier=review; the PR body must list every CI line changed\n'
fi

step "guard: zero dependencies"
if ! git diff --quiet "$base" -- go.mod go.sum; then
  git diff "$base" -- go.mod go.sum >&2
  fail "go.mod/go.sum changed — the zero-dependency rule is not negotiable"
fi

step "guard: no weakened tests"
# Heuristic, and honest about it: on touched _test.go files, assert that the
# number of assertion-ish lines removed does not exceed the number added, and
# that no skip was introduced. A genuine assertion rewrite trips this too — the
# run is then supposed to say so in the issue instead of reshaping the gate.
assert_re='t\.(Fatal|Fatalf|Error|Errorf|Skip|Skipf|Fail|FailNow)|require\.|assert\.|want[A-Z]'
removed=$(git diff "$base" -- '*_test.go' | grep -E '^-' | grep -v '^---' | grep -cE "$assert_re" || true)
added=$(git diff "$base" -- '*_test.go' | grep -E '^\+' | grep -v '^+++' | grep -cE "$assert_re" || true)
skips=$(git diff "$base" -- '*_test.go' | grep -E '^\+' | grep -v '^+++' | grep -cE 't\.Skip' || true)
if [ "${skips:-0}" -gt 0 ]; then
  git diff "$base" -- '*_test.go' | grep -E '^\+.*t\.Skip' >&2
  fail "a t.Skip was added — a skipped test is a deleted test with better manners"
fi
if [ "${removed:-0}" -gt "${added:-0}" ]; then
  printf 'removed assertions: %s / added: %s\n' "$removed" "$added" >&2
  git diff "$base" -- '*_test.go' | grep -E '^-' | grep -v '^---' | grep -E "$assert_re" >&2
  fail "assertions were removed without replacements"
fi
printf 'ok (removed %s / added %s assertions)\n' "${removed:-0}" "${added:-0}"

# ----------------------------------------------------------------- 2. build
step "gofmt"
bad=$(gofmt -l . | grep -v '^testdata/' || true)
[ -n "$bad" ] && fail "unformatted: $(echo "$bad" | tr '\n' ' ')"
printf 'ok\n'

step "go build ./..."
go build ./... 2>&1 | tail -20 | grep . && fail "go build red"
printf 'ok\n'

step "go vet ./..."
go vet ./... 2>&1 | tail -20 | grep . && fail "go vet red"
printf 'ok\n'

# ------------------------------------------------------------------ 3. tests
step "go test -race -count=1 ./..."
out=$(go test -race -count=1 ./... 2>&1)
printf '%s\n' "$out" | grep -E '^(FAIL|--- FAIL)' >&2
if printf '%s\n' "$out" | grep -qE '^(FAIL|--- FAIL)'; then
  fail "the full race suite is red"
fi
printf 'ok (%s packages)\n' "$(printf '%s\n' "$out" | grep -c '^ok')"

step "action smoke"
scripts/action-smoke.sh 2>&1 | tail -5 || fail "action-smoke.sh red"
printf 'ok\n'

step "fixture modules"
for d in testdata/fixtures/*/; do
  [ -f "$d/go.mod" ] || continue
  out=$(cd "$d" && go build ./... 2>&1 && go vet ./... 2>&1) || true
  [ -n "$out" ] && { printf '%s\n' "$out" >&2; fail "fixture $d fails build/vet"; }
done
printf 'ok\n'

printf '\n%sGATE GREEN%s\n' "$green" "$off"
