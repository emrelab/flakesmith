#!/usr/bin/env bash
# Deterministic next-issue picker for the autonomous loop (docs/AUTO.md §2).
#
# The picker is a script, not a judgement: an agent that chooses its own next
# task is the drift this system exists to prevent.
#
# Order: P1 first, then tier (auto:safe before auto:review), then oldest. Safe
# work ships by itself and keeps `main` moving; review work piles up in the PR
# queue where a human drains it.
#
# auto:pr is excluded for the same reason auto:wip is: a review-tier run ends by
# opening a PR and dropping auto:wip, so without this label the same issue would
# be picked again on the very next call and every tick would stack another PR on
# top of the one a human has not read yet.
#
# Exit: 0 = picked (TSV on stdout), 3 = queue empty.
# stdout: <number>\t<labels>\t<title>\t<url>

set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 1

command -v gh >/dev/null 2>&1 || { echo "gh not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq not found" >&2; exit 1; }

read -r -d '' PICK <<'JQ' || true
  map(. + {
    pri:  (if ([.labels[].name] | index("P1")) then 0 else 1 end),
    tier: (if ([.labels[].name] | index("auto:safe")) then 0 else 1 end)
  })
  | sort_by([.pri, .tier, .createdAt])
  | .[0]
  | if . == null then empty
    else [ (.number|tostring),
           ([.labels[].name] | map(select(startswith("auto:") or . == "P1")) | join(",")),
           .title,
           .url ] | @tsv
    end
JQ

# --selftest: proves every branch (P1 wins, safe beats older review, wip is
# unselectable, empty queue prints nothing) without touching GitHub.
if [ "${1:-}" = "--selftest" ]; then
  f1='[{"number":3,"title":"review-old","url":"u3","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"}]},{"number":14,"title":"safe-new","url":"u14","createdAt":"2026-02-01T00:00:00Z","labels":[{"name":"auto:safe"}]},{"number":9,"title":"p1-review","url":"u9","createdAt":"2026-03-01T00:00:00Z","labels":[{"name":"auto:review"},{"name":"P1"}]}]'
  a=$(printf '%s' "$f1" | jq -r "$PICK")
  [ "$(printf '%s' "$a" | cut -f1)" = "9" ] || { echo "FAIL: P1 must win (got: $a)" >&2; exit 1; }
  f2='[{"number":3,"title":"review-old","url":"u3","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"}]},{"number":14,"title":"safe-new","url":"u14","createdAt":"2026-02-01T00:00:00Z","labels":[{"name":"auto:safe"}]}]'
  b=$(printf '%s' "$f2" | jq -r "$PICK")
  [ "$(printf '%s' "$b" | cut -f1)" = "14" ] || { echo "FAIL: auto:safe must beat older auto:review (got: $b)" >&2; exit 1; }
  f3='[{"number":3,"title":"review-old","url":"u3","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"}]},{"number":14,"title":"safe-new","url":"u14","createdAt":"2026-02-01T00:00:00Z","labels":[{"name":"auto:safe"}]}]'
  c=$(printf '%s' "$f3" | jq -r 'map(select([.labels[].name] | index("auto:wip") | not)) | '"$PICK")
  [ "$(printf '%s' "$c" | cut -f1)" = "14" ] || { echo "FAIL: no-wip filter broke (got: $c)" >&2; exit 1; }
  d=$(printf '%s' '[{"number":3,"title":"only-wip","url":"u3","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"},{"name":"auto:wip"}]}]' | jq -r 'map(select([.labels[].name] | index("auto:wip") | not)) | '"$PICK")
  [ -z "$d" ] || { echo "FAIL: a wip issue must not be selectable (got: $d)" >&2; exit 1; }
  # An issue whose PR is already open is the review tier's resting state. It is
  # still open and still labelled auto:review, so nothing but this filter stops
  # the picker from handing it out again on the next call.
  f5='[{"number":13,"title":"pr-open","url":"u13","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"},{"name":"auto:pr"}]},{"number":19,"title":"still-queued","url":"u19","createdAt":"2026-02-01T00:00:00Z","labels":[{"name":"auto:review"}]}]'
  g=$(printf '%s' "$f5" | jq -r 'map(select([.labels[].name] | index("auto:pr") | not)) | '"$PICK")
  [ "$(printf '%s' "$g" | cut -f1)" = "19" ] || { echo "FAIL: an issue with an open PR must be skipped (got: $g)" >&2; exit 1; }
  h=$(printf '%s' '[{"number":13,"title":"pr-open","url":"u13","createdAt":"2026-01-01T00:00:00Z","labels":[{"name":"auto:review"},{"name":"auto:pr"}]}]' | jq -r 'map(select([.labels[].name] | index("auto:pr") | not)) | '"$PICK")
  [ -z "$h" ] || { echo "FAIL: an issue whose only label is auto:pr must not be selectable (got: $h)" >&2; exit 1; }
  e=$(printf '%s' '[]' | jq -r "$PICK")
  [ -z "$e" ] || { echo "FAIL: empty queue must print nothing (got: $e)" >&2; exit 1; }
  echo "selftest ok"
  exit 0
fi

rows=$(gh issue list --state open --limit 200 \
  --search 'label:auto:safe,auto:review -label:auto:wip -label:auto:blocked -label:auto:pr' \
  --json number,title,labels,url,createdAt 2>&1) || {
  echo "$rows" >&2
  exit 1
}

line=$(printf '%s' "$rows" | jq -r "$PICK")
if [ -z "$line" ]; then
  echo "KUYRUK BOS — auto:safe/auto:review etiketli, wip olmayan açık issue yok." >&2
  exit 3
fi
printf '%s\n' "$line"
