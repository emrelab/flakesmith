#!/usr/bin/env bash
# Which ref an autonomous run must branch from (docs/AUTO.md §3).
#
# The hazard the runbook guards against is a STALE local main — a checkout that
# never fetched, so a branch cut from it is missing other people's work. Local
# main being AHEAD is the opposite condition: it holds commits (the harness, the
# rules) that origin/main does not, and branching from origin/main there would
# delete scripts/auto/ and docs/AUTO.md out of the worktree — the gate could not
# even run.
#
# So: fast-forward state (ahead, not behind) -> local main; anything else ->
# origin/main. Behind or diverged must never be treated as "fine", hence the
# explicit exit 4 for divergence.
#
# stdout: the ref to use ("main" or "origin/main"). Exit 4 = diverged.

set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 1

git rev-parse --verify --quiet origin/main >/dev/null || { echo "main"; exit 0; }

ahead=$(git rev-list --count origin/main..main)
behind=$(git rev-list --count main..origin/main)

if [ "$ahead" -eq 0 ]; then
  echo "origin/main"
elif [ "$behind" -eq 0 ]; then
  echo "main"
else
  echo "DIVERGED: main is ${ahead} ahead / ${behind} behind origin/main — a human must rebase." >&2
  exit 4
fi
