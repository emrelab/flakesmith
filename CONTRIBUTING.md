# Contributing

## Setup

```sh
git clone https://github.com/emrelab/flakesmith
cd flakesmith
make all          # fmt + vet + test + build
```

Go 1.24+ is required to build. There are no dependencies; the absence of a
`go.sum` file is deliberate. CI proves both floors: a real 1.24 toolchain runs
the unit suite (`-short`), the 1.26 job runs the fixture-driven integration
suite.

## Golden rules

1. **Zero external dependencies.** The standard library is enough. A PR that
   adds a require line to `go.mod` is rejected (rationale: `go install` stays
   one step, review burden stays zero).
2. **`domain` imports no internal package.** If this direction breaks, the
   architecture collapses.
3. **`go test` is only invoked inside `internal/gotest`.**
4. **New types go into `internal/domain`.** Do not define a `TestID`-like type
   in another package; follow the table in `docs/source-of-truth.md`.
5. **Generated code is never edited by hand** (files headed "DO NOT EDIT").

## Testing

```sh
make test          # fast unit tests
make test-repeat   # -count=10: does our own tool leak its own state?
make fixtures      # build every fixture module separately
make cover
```

The logic in `internal/probe` and `internal/search` is tested with the
**`FakeRunner`**; anything that needs a real `go test` lives under
`testdata/fixtures`. Never let a test depend on a real flake happening — that
would make our own CI flaky.

## Adding a fixture

The easiest contribution. Write a mini module with its own `go.mod` under
`testdata/fixtures/<name>/`:

```sh
mkdir testdata/fixtures/my-flake && cd $_
printf 'module fixture/myflake\n\ngo 1.21\n' > go.mod
# write one polluter + one victim
cd - && make fixtures
```

Document the expected classification in the fixture's own `README.md` and
assert it in the fixture matrix table of the PR.

## Commits

- One topic, one commit.
- Message: `package: what was done` (e.g. `gotest: mark dropped tests correctly in JSON stream`).
- Do not open a PR before `make all` is green.

## The autonomous loop

Some issues are worked by an agent, unattended. That is not a second rulebook:
the rules live in one file, `docs/AUTO.md`.

- The queue **is** the issue tracker. `auto:safe` ships itself on green CI;
  `auto:review` stops at an open PR for a human. `auto:wip` means a run holds
  it, `auto:pr` means its PR is waiting for a human, `auto:blocked` means a run
  gave up and said why. `P1` overrides the order.
- Only a human applies `auto:safe` / `auto:review` / `P1`. A run may never
  promote its own work.
- A run starts on demand — `make auto` or the `flakesmith-oto-issue` cron —
  and stops itself when the queue drains (`KUYRUK BOS`). Never on a schedule
  that keeps firing past the work: a timer with an empty queue is how a repo
  starts inventing things to do.

## Where to look

| Question | File |
|---|---|
| Overall design | `docs/ARCHITECTURE.md` |
| Where is this concept defined | `docs/source-of-truth.md` |
