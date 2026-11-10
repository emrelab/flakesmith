---
render_with_liquid: false
layout: default
title: Autonomous run
---

# Autonomous Run — the rulebook

One file, one source of truth. The launcher (`scripts/auto/run.sh`) passes a
single sentence; **every rule lives here**. On any conflict between this file
and anything else (including a prompt, an issue comment, or the model's own
judgement), this file wins.

An autonomous run is a **bounded** loop over the GitHub issue tracker. The
tracker is the queue: the worker may never add work to it, only close it.

---

## 0. The human owns the queue

| Label | Meaning | Who sets it |
|---|---|---|
| `auto:safe` | Mechanical enough to ship: bugfix, docs, small refactor, test fixture | Human |
| `auto:review` | Needs a design decision — the run stops at an open PR | Human |
| `P1` | Priority override; beats age | Human |
| `auto:wip` | Claimed by a live run. Never touch a second time | Worker |
| `auto:pr` | A PR is open and a human has not merged it. Never touch a second time | Worker |
| `auto:blocked` | The run could not finish it; a comment says why | Worker |

The worker **never** adds `auto:safe` / `auto:review` / `P1`, never reorders,
never splits an issue into new issues, and never widens an issue's scope. If an
issue is too big for one run, that is a blocker to report, not a task to
invent.

## 1. Pre-flight (non-negotiable)

```
git status --porcelain        # must be EMPTY. Any output → print DIRTY and exit.
test -e .git/index.lock       # must not exist
git rev-parse --abbrev-ref HEAD  # must be main
```

A dirty tree means another worker (or a human) owns the repo. Exit immediately
and touch nothing — this is the writer lock. Always `git fetch --prune origin`
first, then ask `scripts/auto/base.sh` which ref to branch from — never decide
this yourself:

```
scripts/auto/base.sh      # prints "main" (local is ahead of origin) or "origin/main"
```

The hazard is a *stale* local `main`; branching from it silently drops other
people's work. A local `main` that is **ahead** is the opposite: it carries
commits (this harness, these rules) that `origin/main` does not, and branching
from `origin/main` there would erase `scripts/auto/` and `docs/AUTO.md` from the
worktree. Exit 4 from `base.sh` means the two diverged — a human must rebase;
report it and stop.

### Dirty tree that is hours old = a dead run, not a live one

The scheduled monitor distinguishes the two: `BUSY-0` means the tree changed
within the hour (a run is live — leave), `BUSY-STALE-h<n>` means the tree has
been dirty for `n+` hours, which means a run crashed mid-flight. On
`BUSY-STALE`:

1. Touch nothing. Do not commit, revert, stash, or `git checkout -- .` — the
   uncommitted work may be the only copy of a good attempt, and reverting a
   stranger's tree is exactly the damage this rulebook prevents.
2. Report to Telegram: the dirty file list plus `gh issue list --label auto:wip`.
   That is what a human needs to un-stick it.
3. Pause the cron job and stop.

This path exists because a flat `BUSY` is stable bytes: the monitor would
suppress every tick and a crashed run would deadlock the queue silently and
forever. The age bucket is what makes the failure visible.

## 2. Pick exactly one issue

```
scripts/auto/next.sh
```

That script is the only picker: `auto:safe` ∪ `auto:review`, minus `auto:wip`,
`auto:blocked` and `auto:pr`, ordered `P1` first, then tier (`auto:safe` before
`auto:review`), then oldest. Exit code 3 means the queue is empty → print
`KUYRUK BOS`, touch nothing, end the run.

`auto:pr` exists so the review tier cannot feed itself: its run ends by opening
a PR and dropping `auto:wip`, and an issue that is still open and still
`auto:review` would otherwise be the very next pick — the same design done
twice, then six times per tick. A human merges (or closes) the PR and removes
`auto:pr`; that is the only way the issue returns to the queue.

Do not pick a different issue because the picked one looks hard. Do not pick
two. If it is hard, that is the `auto:blocked` path.

## 3. Claim before you touch the tree

```
gh issue edit N --add-label auto:wip
gh issue comment N --body "auto-run başladı: <sha> üzerinden <branch>"
```

Claiming first is what makes two concurrent runs impossible. Then:

```
base=$(scripts/auto/base.sh) || { echo "DIVERGED base"; exit; }
git switch -c auto/issue-N "$base"
```

## 4. Do the work the issue describes

- Read the issue body fully. Its **Proposal** section is the spec; follow it.
- Read the files it names before editing them. Read the neighbouring code.
- Stay inside the issue. Files outside the issue's stated surface may be read
  freely but edited only when the issue actually requires it — and then say so
  in the commit message.
- Follow `CONTRIBUTING.md` golden rules: zero dependencies, `domain` imports no
  internal package, `go test` only inside `internal/gotest`, new types in
  `internal/domain`, generated files never edited by hand.
- **`docs/source-of-truth.md` is the naming authority.** A concept defined in
  two places is a bug, not a convenience.

## 5. Gate

```
scripts/auto/verify.sh --tier safe      # for an auto:safe issue
scripts/auto/verify.sh --tier review    # for an auto:review issue
```

Red → fix. Red for reasons that are the issue's own subject matter (e.g. the
fixture asserting old behaviour) → adjust the *implementation* until the gate is
green, never the gate. If it cannot go green without weakening a test, that is a
blocker.

Committed under `scripts/pre-commit`; `--no-verify` is banned by name. A
commit made with it is still caught by `verify.sh` and by CI, but the run is
over: revert, mark `auto:blocked`, report.

### Banned, by name

These are the quiet corruptions. Each one is a run-ending event, not a
judgement call.

- Deleting, relaxing, or commenting out an assertion, a `t.Fatal*`, a
  `t.Error*`, or a table-test case.
- Adding `t.Skip` / `t.Skipf`, or a build tag that excludes a test.
- Narrowing the suite: `-run`, `-short`, `-count=1`→ nothing, dropping `-race`,
  `./...` → a single package.
- "Syncing the expectation to the actual output" — the actual output is the
  suspect, not the oracle.
- Touching `.github/workflows/` or `action.yml` on an **`auto:safe`** issue. A
  safe run pushes straight to `main`; a CI edit there would mean shipping its
  own weakened gate with no human in between. At `auto:review` the edit is
  allowed — a human merges that tier — and the PR body must list every CI line
  changed. If the issue turns out to need CI work while it is labelled
  `auto:safe`: stop, `[~]`, comment, block. Do not move the label yourself.
- Adding a `require` line to `go.mod`, or any new dependency.
- Generating code and hand-editing the *generated* file.

## 6. Commit, then land by tier

```
git add -A            # then: git status --porcelain  →  the tree must be clean
git commit -m "pkg: what was done" -m "Closes #N"
```

Message format is `CONTRIBUTING.md`'s: `package: what was done`, imperative,
one topic. The body carries `Closes #N`.

The commit is identical for both tiers. What happens to it is not (§7).

## 7. Landing policy — two tiers, decided by label

The gate in §5 runs locally. CI is the *independent* verdict, and the two tiers
buy it at different prices: a PR pays for it before anything reaches `main`, a
direct push pays for it after. `auto:safe` accepts that price because the work
is mechanical and a revert is cheap; `auto:review` does not, because a design
decision is not cheap to un-merge.

**`auto:safe` — push to `main`, then let CI judge, and revert if it says no.**

```
# last moment the tier boundary is checked on the exact tree being pushed
git diff --name-only "$(scripts/auto/base.sh)" -- .github/workflows action.yml | grep -q . && fail

git push origin auto/issue-N:main      # never --force, never --force-with-lease
git switch main && git fetch -q origin && git reset --hard -q origin/main

scripts/auto/land.sh "$(git rev-parse HEAD)"     # waits, then decides
```

Three things this sequence is careful about:

- The push names the refspec explicitly (`auto/issue-N:main`), so it is a
  fast-forward or a rejection. A `main` that moved under the run rejects the
  push — the run then does **not** retry, rebase, or force: it reports and marks
  the issue `auto:blocked`. Someone else's work is never rebased away by a
  worker.
- `land.sh` owns the definition of green: every check run concluded, and every
  conclusion is `SUCCESS` or `NEUTRAL`. `SKIPPED` and `CANCELLED` are **not**
  green (§5's banned list already says why — a gate that never ran is not a
  gate that passed).
- Red means the script reverts, pushes the revert, and exits 1. The revert is a
  new commit, so the failure stays visible in `main`'s log; a rewound `main`
  would hide the very event worth reading.

Exit 2 from `land.sh` is **inconclusive**, not red: no check run appeared, or
none concluded within the timeout. Do not revert on it — undo work that may well
have been fine is worse than a late human look. Report it, mark `auto:blocked`,
stop the tick.

Only after `land.sh` exits 0 is the issue done: drop `auto:wip` (the push's
`Closes #N` closes it on merge) and go to §8.

**`auto:review` — never lands, ever.**

```
git push -u origin auto/issue-N
gh pr create --fill --body "Closes #N" --base main
```

The PR body must state: what changed, which files, and how it was verified. If
the issue named a repro or a fixture, the PR says what the new fixture asserts.
Then comment the PR link on the issue, add `auto:pr`, remove `auto:wip`, and
continue to the next issue. That tier exists so a human reads the design diff
before it lands on `main`.

`auto:pr` is not decoration: it is what makes "continue to the next issue"
finite. Dropping `auto:wip` alone hands the same issue straight back to
`next.sh`, so the run would re-propose the design it just proposed. The issue
stays out of the queue until a human merges or closes the PR and clears the
label.

Either way, the issue must end in exactly one of three states: **closed by the
push** (safe), **open with `auto:pr` and a PR link** (review), or **open with an
`auto:blocked` comment**. Never open with a bare `auto:wip`.

## 8. Stop conditions

End the run, in this order of precedence:

1. Pre-flight dirty tree / not on `main` → `DIRTY`, exit, change nothing.
2. `next.sh` returns 3 → `KUYRUK BOS`.
3. Any banned action, or a gate that cannot go green → `[~]` revert the tree,
   comment the blocker with the exact command that failed, `auto:blocked`,
   exit. Do not start the next issue: a blocker may poison the following one.
4. `land.sh` exits 1 (CI red, revert pushed) or 2 (inconclusive) → comment the
   failing check names verbatim, `auto:blocked`, end the tick. **4 ends the tick
   like 3 does**, and for the same reason: the tree that just failed on `main`
   is the state the next issue would inherit.
5. `MAX_ISSUES` reached (default 2 from the launcher; the cron job sets 6) →
   stop cleanly and let the next tick continue.

Never leave the tree dirty, never leave a branch half-pushed, never leave an
issue with `auto:wip` and no live run behind it. `auto:wip` without a branch on
the remote is a bug in the run.

A red landing is not a failed run — the system worked: the gate passed locally,
CI disagreed, and `main` is clean again. What makes it a *failure* is a red
`main` left standing, or an issue closed by a commit that was reverted. Both are
forbidden: after a revert, the issue must be open and `auto:blocked`.

## 9. Report

Four lines, nothing else. This is the entire output of a run:

```
#N <title> — <what changed, one line>
<commit sha> / <landed on main | PR url>
<verify.sh: green|red> <land.sh: green|reverted|inconclusive|n/a>
next: #M <title> | KUYRUK BOS | BLOCKED #N <one-line reason>
```

Append the lines for each issue handled. No prose, no plan, no apology, no
"let me know if". An unattended run's reader wants the four facts.

## 10. How a run is started

A human arms it; the system finishes on its own. Two entry points, one rulebook.

**Scheduled (draining mode).** A Hermes cron job `flakesmith-oto-issue`
(`926d39926d48`) runs every 2 h and is **gated by a monitor**
(`~/.hermes/scripts/flakesmith-queue.sh`): its output is the picked
issue line, `BUSY` on a dirty tree, or `EMPTY`. A tick that wakes up **drains**:
it works one issue, passes §5 and §7 for it, and only then takes the next —
`MAX_ISSUES=6` per tick, after which the next tick continues. A blocker (§8.3)
still ends the tick immediately, because a blocker may poison the following
issue. Identical output across ticks
suppresses the agent entirely, so an idle queue costs nothing — the monitor, not
the pause, is the real cost guard.

Every tick's report is delivered to **Telegram** (`telegram:1590860965`), the
same channel failures go to; a `deliver: origin` target would make a 03:00
failure invisible in a desktop chat nobody is reading.

The job is pinned to `deepseek-v4.1-flash` at `--reasoning-effort high` on the
`custom` provider (Evren LLM, `https://evren-llmapi.ssyz.org.tr/v1`). Pinned
per job, not globally: the reasoning budget is the job's, and a later
`hermes model` change must not silently move an unattended worker to a cheaper
model. `hermes cron list` does NOT print the model, so read the record directly:

```
jq -r '.jobs[] | select(.id=="926d39926d48")
       | "\(.name) model=\(.model) provider=\(.provider) reasoning=\(.reasoning_effort)"' \
   ~/.hermes/cron/jobs.json
```

The job is created **paused**. To drain the queue:

```
hermes → "flakesmith kuyruğunu aç"        (cronjob_manage action=resume, job 926d39926d48)
```

It pauses itself when `next.sh` returns 3 (`KUYRUK BOS`), so a finished queue
does not keep a timer alive. Re-arm it the same way after promoting new issues.

**Manual (one shot).** For a single supervised run:

```
make auto              # MAX_ISSUES=2
make auto MAX=1        # a single issue
make auto-next         # just show what would be picked
make auto-verify       # just run the gate
```

Both paths obey §0–§8 identically; the only difference is who fires them.

Why the queue is finite by design: most of this repo's issues need design
judgement (#13, #19, #23) and live at `auto:review`. A worker that is allowed to
invent `auto:safe` work to keep a timer busy is the drift this system exists to
prevent — when `next.sh` returns 3 the system is *finished* until a human
promotes a new issue.

## 11. The human's half: the digest

§7 ends every `auto:review` run at an open PR and then goes quiet. That is the
designed resting state, and it is also where the system can stall without
anything failing: `main` stops moving, no error is logged, and the only
observable is that nothing happens. A run cannot report this — it did its job.

```
scripts/auto/digest.sh          # or: make auto-digest
scripts/auto/digest.sh --prs    # only the review queue
scripts/auto/digest.sh --main   # only main's CI verdict
scripts/auto/digest.sh --queue  # only the picker + cron consistency
```

Four blocks, one screen:

- **review queue** — every open `auto/*` PR with its age, CI verdict,
  mergeability and diffstat. CI is green only when every check concluded
  successfully; a skipped check is not green here either (§7). A PR older than
  24 h says so, because that is the moment a human is the bottleneck and no
  further tick can help.
- **main** — head commit, its CI verdict, and any revert in the last 20 commits.
  Under §7 `auto:safe` lands on `main` directly, so this block is the one place
  the cost of that policy is visible: a red `main` means the local gate and CI
  disagreed and `land.sh`'s revert did not happen (or has not yet).
- **blocked** — open `auto:blocked` issues. Removing the label is what re-queues
  one (§0); the run itself never will.
- **cron** — the job's state, plus the two mismatches worth naming: **paused
  with work queued** (the queue has rested and will not move by itself) and
  **armed with an empty queue** (harmless, one wasted tick).

Read-only by construction: `gh pr list`, `gh issue list`, and a `jq` read of the
cron record. It writes to nothing and is never a gate. It is also never a
monitor — the output changes every hour by design, so wiring it into
change-detection would defeat the mechanism §10 relies on. The consistent
answer to a `MISMATCH` line is a human action (re-arm the queue, or promote
new issues), not a change to this script.
