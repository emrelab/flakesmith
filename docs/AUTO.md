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
and branch from `origin/main`, never from a local `main` that may be stale.

## 2. Pick exactly one issue

```
scripts/auto/next.sh
```

That script is the only picker: `auto:safe` ∪ `auto:review`, minus `auto:wip`
and `auto:blocked`, ordered `P1` first, then tier (`auto:safe` before
`auto:review`), then oldest. Exit code 3 means the queue is empty → print
`KUYRUK BOS`, touch nothing, end the run.

Do not pick a different issue because the picked one looks hard. Do not pick
two. If it is hard, that is the `auto:blocked` path.

## 3. Claim before you touch the tree

```
gh issue edit N --add-label auto:wip
gh issue comment N --body "auto-run başladı: <sha> üzerinden <branch>"
```

Claiming first is what makes two concurrent runs impossible. Then:

```
git switch -c auto/issue-N origin/main
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
  safe run auto-merges; a CI edit there would mean merging its own weakened
  gate. At `auto:review` the edit is allowed — a human merges that tier — and
  the PR body must list every CI line changed. If the issue turns out to need
  CI work while it is labelled `auto:safe`: stop, `[~]`, comment, block. Do not
  move the label yourself.
- Adding a `require` line to `go.mod`, or any new dependency.
- Generating code and hand-editing the *generated* file.

## 6. Commit, push, PR

```
git add -A            # then: git status --porcelain  →  the tree must be clean
git commit -m "pkg: what was done" -m "Closes #N"
```

Message format is `CONTRIBUTING.md`'s: `package: what was done`, imperative,
one topic. The body carries `Closes #N`.

```
git push -u origin auto/issue-N
gh pr create --fill --body "Closes #N" --base main
```

The PR body must state: what changed, which files, and how it was verified. If
the issue named a repro or a fixture, the PR says what the new fixture asserts.

## 7. Merge policy — two tiers, decided by label

**`auto:safe`** — wait for CI, then merge:

```
gh pr checks --watch --interval 30      # green required
gh pr merge --squash --delete-branch
```

Green means green: `test`, `oldest`, and `self-hunt` all pass. A skipped or
cancelled check is not green. On red: do not merge, revert to the previous
state, mark `auto:blocked`, put the failing output in a comment.

Before merging, re-assert the tier boundary on the remote branch — the gate ran
locally, so this is the independent check:

```
git diff --name-only origin/main...HEAD | grep -E '^\.github/|^action\.yml$' && fail
```

Any output means a CI file is in the diff: shut it down, whether or not
`verify.sh` said green.

**`auto:review`** — never merge, ever. Comment the PR link on the issue, remove
`auto:wip`, and continue to the next issue. That tier exists so a human reads
the design diff before it lands on `main`.

Either way, the issue must end in one of exactly two states: **closed by the
merge** or **open with an `auto:blocked` comment**.

## 8. Stop conditions

End the run, in this order of precedence:

1. Pre-flight dirty tree / not on `main` → `DIRTY`, exit, change nothing.
2. `next.sh` returns 3 → `KUYRUK BOS`.
3. Any banned action, or a gate that cannot go green → `[~]` revert the tree,
   comment the blocker with the exact command that failed, `auto:blocked`,
   exit. Do not start the next issue: a blocker may poison the following one.
4. `MAX_ISSUES` reached (default 2, set by the launcher) → stop cleanly.

Never leave the tree dirty, never leave a branch half-pushed, never leave an
issue with `auto:wip` and no live run behind it. `auto:wip` without a branch on
the remote is a bug in the run.

## 9. Report

Four lines, nothing else. This is the entire output of a run:

```
#N <title> — <what changed, one line>
<commit sha> / <PR url or "merged">
<verify.sh: green|red> <merge state>
next: #M <title> | KUYRUK BOS | BLOCKED #N <one-line reason>
```

Append the lines for each issue handled. No prose, no plan, no apology, no
"let me know if". An unattended run's reader wants the four facts.

## 10. Manual trigger

There is deliberately **no cron**. A human starts a run:

```
make auto              # MAX_ISSUES=2
make auto MAX=1        # a single issue
make auto-next         # just show what would be picked
make auto-verify       # just run the gate
```

Reason: this repo's issues need design judgement in the majority of cases
(#13, #19, #23), and a timer would run out of `auto:safe` work and start
inventing it. The queue is finite by construction; when `next.sh` returns 3 the
system is *finished* until a human promotes a new issue.
