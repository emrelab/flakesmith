#!/usr/bin/env bash
# Manual launcher for one autonomous run (docs/AUTO.md §10).
#
#   scripts/auto/run.sh [MAX_ISSUES]     default 2
#
# There is no cron on purpose: most of this repo's open issues need a design
# decision, and a timer with a finite queue eventually invents work. A human
# starts the run; the run is bounded and reports four lines per issue.
#
# --yolo is required: an unattended run cannot answer approval prompts. That is
# why the pre-flight below refuses to start on a dirty tree — the blast radius
# of a mis-aimed autonomous commit is exactly the repo it runs in.

set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 1

MAX=${1:-2}
case "$MAX" in ''|*[!0-9]*) echo "MAX_ISSUES must be a number" >&2; exit 2 ;; esac

command -v hermes >/dev/null 2>&1 || { echo "hermes not on PATH" >&2; exit 1; }
command -v gh >/dev/null 2>&1     || { echo "gh not on PATH" >&2; exit 1; }

# Writer lock, same check the runbook performs: a dirty tree means someone else
# owns this checkout right now.
if [ -n "$(git status --porcelain)" ]; then
  echo "DIRTY: working tree has changes — refusing to start." >&2
  git status --short >&2
  exit 3
fi
if [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then
  echo "not on main ($(git rev-parse --abbrev-ref HEAD)) — refusing to start." >&2
  exit 3
fi
if [ -e .git/index.lock ]; then
  echo ".git/index.lock exists — another git process is live." >&2
  exit 3
fi

git fetch --prune origin || { echo "git fetch failed" >&2; exit 1; }

if ! scripts/auto/next.sh >/dev/null; then
  echo "KUYRUK BOS — nothing labelled auto:safe or auto:review is open." >&2
  exit 0
fi

PROMPT="Read docs/AUTO.md in full first; it is the only source of truth and it wins on any conflict. Execute ONE autonomous run with MAX_ISSUES=${MAX}. Report exactly the four lines per issue defined in §9 and nothing else."

printf 'starting autonomous run: MAX_ISSUES=%s at %s\n' "$MAX" "$(date '+%F %T')"

exec hermes chat --oneshot --yolo \
  --max-turns 400 \
  --run-budget 7200 \
  -q "$PROMPT"
