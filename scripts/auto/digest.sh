#!/usr/bin/env bash
# digest.sh — the human's side of the autonomous loop, in one screen.
#
# Why this exists: docs/AUTO.md §7 ends every `auto:review` run at an open PR
# and then goes quiet. Nothing in the system tells a human that the review
# queue has become the bottleneck — the run is *working as designed* while
# `main` stops moving. The measured failure is not hypothetical: two review
# PRs sat green and mergeable while the next tick kept finding ready work
# behind them.
#
# Read-only by construction: `gh pr list`, `gh issue list`, one `jq` read of the
# cron record. It prints nothing about itself and never writes to the repo, the
# tracker, or the cron state. An operator tool like scripts/test-cost.sh, not a
# gate and not a monitor — its output changes every hour, so it must never be
# wired into a change-detection monitor.
#
# Usage:
#   scripts/auto/digest.sh            # the whole picture
#   scripts/auto/digest.sh --prs      # only the review queue
#   scripts/auto/digest.sh --queue    # only the picker + cron consistency
#
# Exit: always 0 unless gh/jq is missing (1). A reporter that exits non-zero on
# "nothing to report" trains its reader to ignore it.

set -uo pipefail
cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" || exit 1

for c in gh jq; do
  command -v "$c" >/dev/null 2>&1 || { echo "$c not found" >&2; exit 1; }
done

JOBS="${HERMES_CRON_JOBS:-$HOME/.hermes/cron/jobs.json}"
JOB_ID="926d39926d48"
WHAT="${1:-all}"

now=$(date -u +%s)

# Hours between a GitHub timestamp and now, one decimal. `date -jf` is BSD,
# `date -d` is GNU; the repo runs on both, so try in that order.
age_h() {
  local ts=$1 epoch
  epoch=$(date -juf '%Y-%m-%dT%H:%M:%SZ' "$ts" +%s 2>/dev/null ||
          date -ud "$ts" +%s 2>/dev/null) || { echo '?'; return; }
  awk -v a="$epoch" -v b="$now" 'BEGIN { printf "%.1f", (b - a) / 3600 }'
}

# CI verdict for one PR: green only when every check concluded successfully.
# Anything pending is "running"; a skipped or cancelled check is NOT green
# (docs/AUTO.md §7) and must never read as one here either.
ci_state() {
  printf '%s' "$1" | jq -r '
    if ([.[] | select(.conclusion == null or .conclusion == "")] | length) > 0
    then "ci running"
    elif ([.[] | select((.conclusion | ascii_upcase) != "SUCCESS" and (.conclusion | ascii_upcase) != "NEUTRAL")] | length) > 0
    then "CI RED"
    else "ci green" end'
}

prs() {
  local rows
  rows=$(gh pr list --state open --limit 50 \
    --json number,title,headRefName,createdAt,mergeable,statusCheckRollup,additions,deletions,changedFiles,isDraft 2>/dev/null) || {
    echo "== review queue — gh pr list failed" >&2
    return
  }
  # Only runs' PRs: an autonomous branch is named auto/issue-N by §3.
  rows=$(printf '%s' "$rows" | jq -c '[.[] | select(.headRefName | startswith("auto/"))]')
  local n stale
  n=$(printf '%s' "$rows" | jq 'length')
  echo "== review queue (auto/* PRs) — $n"
  [ "$n" -eq 0 ] && { echo "   empty"; return; }

  stale=0
  printf '%s' "$rows" | jq -r '.[] | [
      ("#" + (.number | tostring)),
      ((.additions | tostring) + "/-" + (.deletions | tostring) + " (" + (.changedFiles | tostring) + "f)"),
      (if .isDraft then "draft" else .mergeable end),
      .headRefName,
      .createdAt,
      .title
    ] | @tsv' | while IFS=$'\t' read -r num statable mergeable head created title; do
    local state h
    state=$(ci_state "$(printf '%s' "$rows" | jq -c --argjson n "${num#\#}" '.[] | select(.number == $n) | .statusCheckRollup // []')")
    h=$(age_h "$created")
    printf '  %-5s %6sh  %-10s %-14s %-14s %s\n' "$num" "$h" "$state" "$mergeable" "$statable" "${title:0:64}"
  done

  stale=$(printf '%s' "$rows" | jq -r --argjson now "$now" \
    '[.[] | select(($now - (.createdAt | fromdateiso8601)) > 86400)] | length')
  if [ "${stale:-0}" -gt 0 ]; then
    echo "  ^ $stale PR(s) older than 24h: §7 ended the run and went quiet — merge or close is the only thing that moves the queue"
  fi

  local drafts
  drafts=$(printf '%s' "$rows" | jq '[.[] | select(.isDraft)] | length')
  [ "${drafts:-0}" -gt 0 ] && echo "  ^ $drafts draft PR(s) will never merge themselves"
}

blocked() {
  local rows n
  rows=$(gh issue list --state open --limit 50 --label auto:blocked \
    --json number,title,url 2>/dev/null) || return
  n=$(printf '%s' "$rows" | jq 'length')
  echo
  echo "== blocked (auto:blocked) — $n"
  [ "$n" -eq 0 ] && { echo "   empty"; return; }
  printf '%s' "$rows" | jq -r '.[] | "  #\(.number) \(.title[0:70])"'
  echo "  ^ each one needs a human decision; removing the label re-queues it (§0)"
}

# main's own CI verdict. Since §7 lets `auto:safe` land on main without a PR,
# main is where a bad commit becomes visible — and a revert pushed by land.sh is
# the outcome worth seeing, not hiding.
mainline() {
  local slug sha body total pending
  echo
  echo "== main"
  slug=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null) || {
    echo "   cannot resolve the repository" >&2; return
  }
  sha=$(git rev-parse origin/main 2>/dev/null || git ls-remote origin refs/heads/main 2>/dev/null | cut -f1)
  [ -n "${sha:-}" ] || { echo "   no origin/main"; return; }
  echo "   head: ${sha:0:12} $(git log -1 --format='%s' "$sha" 2>/dev/null | cut -c1-60)"

  body=$(gh api "repos/$slug/commits/$sha/check-runs?per_page=100" 2>/dev/null) || {
    echo "   ci: unknown (check-runs unreadable)"; return
  }
  total=$(printf '%s' "$body" | jq '.check_runs | length')
  pending=$(printf '%s' "$body" | jq '[.check_runs[] | select(.status != "completed")] | length')
  if [ "${total:-0}" -eq 0 ]; then
    echo "   ci: no check run on this commit"
  elif [ "${pending:-0}" -gt 0 ]; then
    echo "   ci: running ($pending of $total not concluded)"
  else
    local bad
    bad=$(printf '%s' "$body" | jq -r '[.check_runs[] | select((.conclusion // "" | ascii_upcase) != "SUCCESS" and (.conclusion // "" | ascii_upcase) != "NEUTRAL")] | length')
    if [ "${bad:-1}" -eq 0 ]; then
      echo "   ci: green ($total checks)"
    else
      echo "   ci: RED ($bad of $total) — main is broken right now"
      printf '%s' "$body" | jq -r '.check_runs[] | select((.conclusion // "" | ascii_upcase) != "SUCCESS" and (.conclusion // "" | ascii_upcase) != "NEUTRAL") | "     \(.name): \(.conclusion)"'
    fi
  fi

  # A revert is a legitimate outcome (§7), but two in a row is a signal that the
  # local gate and CI disagree systematically — worth a human look, and invisible
  # in any single tick's report.
  local reverts
  reverts=$(git log --format='%s' -20 "$sha" 2>/dev/null | grep -c '^Revert "' || true)
  [ "${reverts:-0}" -gt 0 ] && echo "   last 20 commits contain ${reverts} revert(s)"
}

queue() {
  echo
  local line rc
  line=$(scripts/auto/next.sh 2>/dev/null); rc=$?
  local pending
  pending=$(gh issue list --state open --limit 200 \
    --search 'label:auto:safe,auto:review -label:auto:wip -label:auto:blocked -label:auto:pr' \
    --json number 2>/dev/null | jq 'length' 2>/dev/null || echo '?')
  echo "== queue — $pending waiting"

  if [ $rc -eq 3 ]; then
    echo "   KUYRUK BOS"
  elif [ $rc -ne 0 ]; then
    echo "   picker failed (exit $rc)" >&2
  else
    printf '   next: #%s %s\n' "$(printf '%s' "$line" | cut -f1)" "$(printf '%s' "$line" | cut -f3 | cut -c1-70)"
    [ "${pending:-0}" -gt 1 ] && echo "   then: $((pending - 1)) more behind it"
  fi

  echo
  echo "== cron"
  if [ ! -r "$JOBS" ]; then
    echo "   no cron record at $JOBS — scheduled drain not configured here"
    return
  fi
  local state paused_at
  state=$(jq -r --arg id "$JOB_ID" '.jobs[]? | select(.id == $id) | .state // (if .enabled then "enabled" else "paused" end)' "$JOBS" 2>/dev/null)
  paused_at=$(jq -r --arg id "$JOB_ID" '.jobs[]? | select(.id == $id) | .paused_at // "-"' "$JOBS" 2>/dev/null)
  if [ -z "${state:-}" ]; then
    echo "   job $JOB_ID not in $JOBS"
    return
  fi
  echo "   $JOB_ID: $state (since ${paused_at})"

  # The consistency check this whole section exists for. A paused job is the
  # normal resting state of a *finished* queue (§10: next.sh exit 3 pauses it).
  # Paused with work waiting is the silent-failure case: nothing errors, nothing
  # runs, and the only observable is that the queue does not move.
  if [ "$state" = "paused" ] && ! [ $rc -eq 3 ]; then
    echo "   MISMATCH: paused with ${pending:-?} issue(s) queued — a non-empty queue must not rest, re-arm with: flakesmith kuyruğunu aç"
  elif [ "$state" != "paused" ] && [ $rc -eq 3 ]; then
    echo "   MISMATCH: armed with an empty queue — the next tick will pause itself, harmless but noisy"
  fi
}

case "$WHAT" in
  --prs)   prs ;;
  --queue) queue ;;
  --main)  mainline ;;
  all)     prs; mainline; blocked; queue ;;
  *) echo "usage: digest.sh [--prs|--main|--queue]" >&2; exit 2 ;;
esac

exit 0
