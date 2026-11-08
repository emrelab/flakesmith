#!/usr/bin/env bash
# land.sh — the independent check for a commit pushed straight to main.
#
# Why this exists: an `auto:safe` run lands on main without a PR (docs/AUTO.md
# §7), so the only independent verdict left is CI running on main — and it runs
# *after* the push, which is the whole cost of that policy. This script owns the
# two things a prompt must never decide on its own:
#
#   1. what "green" means, mechanically (all check runs concluded successfully),
#   2. what happens when it is not (revert, push, report) — not "the model
#      judges whether the red is serious".
#
# It never force-pushes, never touches a branch other than to reset local main to
# origin/main, and never rewrites history. The revert is a new commit, so the
# evidence of the failure stays on main's log — a rewound main would hide it.
#
# Usage:
#   scripts/auto/land.sh <sha> [--timeout SECONDS] [--no-revert]
#
#   --timeout N    give up waiting after N seconds (default 2700 = 45m)
#   --no-revert    observe only; never revert (for a supervised run)
#
# Exit: 0 green · 1 red and reverted (or red with --no-revert) ·
#       2 inconclusive (no checks started, or they did not conclude in time).
# 2 is deliberately NOT a failure path: a revert decided on a timeout would undo
# work that may well have been fine.
#
# Testing the red path without breaking a real main:
#   LAND_FIXTURE_JSON=<file>  bypasses gh and reads {"check_runs":[...]} from a
#                             file, so the revert branch can be exercised in a
#                             scratch clone with a local bare origin.
#   LAND_SLUG=owner/repo      bypasses `gh repo view` (a scratch clone has no
#                             GitHub remote to resolve).

set -uo pipefail
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 1

SHA="${1:-}"
[ -n "$SHA" ] || { echo "usage: land.sh <sha> [--timeout SECONDS] [--no-revert]" >&2; exit 2; }
shift
TIMEOUT=2700
REVERT=1
while [ $# -gt 0 ]; do
  case "$1" in
    --timeout) TIMEOUT=${2:-2700}; shift 2 ;;
    --no-revert) REVERT=0; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

for c in gh jq git; do
  command -v "$c" >/dev/null 2>&1 || { echo "$c not found" >&2; exit 2; }
done

SLUG=${LAND_SLUG:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null)} || {
  echo "LAND: cannot resolve the repository" >&2; exit 2
}
[ -n "${SLUG:-}" ] || { echo "LAND: cannot resolve the repository" >&2; exit 2; }

# Green is one definition, stated once: every check run concluded, and every
# conclusion is a pass. SKIPPED and CANCELLED are not passes (docs/AUTO.md §7) —
# treating them as green is how a run quietly merges a gate it never ran.
#
# The check-runs API returns lower-case conclusions (`success`), while gh's
# statusCheckRollup returns upper-case ones. Normalising here once is cheaper
# than a second case bug: the first version of this script read nine green
# checks as RED because of exactly that.
failed_jq='[.check_runs[] | select((.conclusion // "" | ascii_upcase) != "SUCCESS" and (.conclusion // "" | ascii_upcase) != "NEUTRAL")]'

verdict() {
  jq -r '
    .check_runs as $runs
    | ($runs | length) as $n
    | ["total", ($n | tostring)] | @tsv,
      ($runs[] | ["run", .name, (.status // "?"), (.conclusion // "-")] | @tsv)' <<<"$1"
}

poll() {
  # LAND_FIXTURE_JSON exists so the red path can be exercised without pushing a
  # broken commit to a real repository: the revert branch is the only code in
  # this repo that writes to main, so leaving it untested was not an option.
  if [ -n "${LAND_FIXTURE_JSON:-}" ]; then cat "$LAND_FIXTURE_JSON"; return; fi
  gh api "repos/$SLUG/commits/$SHA/check-runs?per_page=100" 2>/dev/null
}

printf 'LAND %s — waiting for CI on main (timeout %ss)\n' "${SHA:0:12}" "$TIMEOUT"
started=$(date -u +%s)
saw_checks=0
while :; do
  body=$(poll)
  if [ -n "$body" ]; then
    total=$(jq -r '.check_runs | length' <<<"$body" 2>/dev/null || echo 0)
    pending=$(jq -r '[.check_runs[] | select(.status != "completed")] | length' <<<"$body" 2>/dev/null || echo 0)
    [ "${total:-0}" -gt 0 ] && saw_checks=1
    if [ "${total:-0}" -gt 0 ] && [ "${pending:-1}" -eq 0 ]; then break; fi
  fi
  elapsed=$(( $(date -u +%s) - started ))
  if [ "$elapsed" -ge "$TIMEOUT" ]; then
    printf 'LAND INCONCLUSIVE: CI did not conclude within %ss (checks seen: %s)\n' "$TIMEOUT" "$saw_checks" >&2
    verdict "${body:-{\"check_runs\":[]}}" >&2
    exit 2
  fi
  # No checks after two minutes means the push triggered nothing — a workflow
  # filter mistake would otherwise look like a slow CI, and the run would wait
  # out the whole timeout to report nothing.
  if [ "$saw_checks" -eq 0 ] && [ "$elapsed" -ge 120 ]; then
    printf 'LAND INCONCLUSIVE: no check run appeared for %s in 120s — did the push trigger ci.yml?\n' "${SHA:0:12}" >&2
    exit 2
  fi
  sleep 20
done

verdict "$body"

failed=$(jq -r "$failed_jq | length" <<<"$body")
if [ "${failed:-1}" -eq 0 ]; then
  printf 'LAND GREEN %s\n' "${SHA:0:12}"
  exit 0
fi

printf 'LAND RED %s — %s check(s) did not pass:\n' "${SHA:0:12}" "$failed" >&2
jq -r '.check_runs[] | select((.conclusion // "" | ascii_upcase) != "SUCCESS" and (.conclusion // "" | ascii_upcase) != "NEUTRAL") | "  \(.name): \(.conclusion)"' <<<"$body" >&2

if [ "$REVERT" -eq 0 ]; then
  printf 'LAND RED (not reverted: --no-revert)\n' >&2
  exit 1
fi

# Revert. The tree must be clean before this point (pre-flight, §1) and the run
# has just pushed $SHA, so `origin/main` is the only state main may hold.
if [ -n "$(git status --porcelain)" ]; then
  printf 'LAND RED: tree is dirty — refusing to revert. Revert by hand:\n  git revert %s\n' "$SHA" >&2
  exit 1
fi

git fetch -q origin || { printf 'LAND RED: git fetch failed — revert by hand\n' >&2; exit 1; }
git switch -q main || { printf 'LAND RED: cannot switch to main — revert by hand\n' >&2; exit 1; }
git reset --hard -q origin/main || { printf 'LAND RED: cannot reset main — revert by hand\n' >&2; exit 1; }

if ! git revert --no-edit "$SHA"; then
  printf 'LAND RED: the revert conflicts — main is still red, resolve by hand:\n  git revert %s\n' "$SHA" >&2
  exit 1
fi

# Not --force, not --force-with-lease: if someone pushed on top meanwhile, this
# is rejected and a human sees it. A forced revert would erase their commit.
if ! git push origin main; then
  printf 'LAND RED: revert commit exists locally but the push was rejected (main moved) — push it by hand\n' >&2
  exit 1
fi

printf 'LAND REVERTED %s — main is clean again; the failure stays in the log as %s\n' \
  "${SHA:0:12}" "$(git rev-parse --short HEAD)"
exit 1
