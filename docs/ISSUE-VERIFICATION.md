---
layout: default
title: Issue verification
---

# Remaining issue verification (2026-11-10)

This change addresses the six issues open after the baseline and regression
artifact fixes. The fixture modules are intentionally faulty examples; their
normal test suites are not expected to pass. Root integration tests copy and
exercise them independently.

| Issue | Result | Regression coverage |
|---|---|---|
| #2 | Advisory package-wide clock/network/database/random call names are printed before probing and carried as static evidence; arguments are never captured. Probes still run. | Aliased-import scan and scheduler test asserting the hint arrives before the probe. |
| #6 | Two-process cross-package reproduction is supported end to end; stage 2 requires a successful stage 1; both directories stay locked throughout the sequence. `init --replay-capsule` can enforce a committed capsule in CI. | Cross-package fixture hunt → shrink → compile refusal → replay 3/3; failed-stage guard; scaffold exit mapping 0→1, 3→0, 6→6, 130→130. |
| #16 | Partial verification produces `suspected`, measured k/n and Wilson 95%. Full N/N remains required for confirmation. Suspected findings cannot enter baseline/flake gate or be compiled. | Independent cache bypass, complete verification rounds, 0/3 vs 2/3 vs 3/3, scanner gate/baseline exclusion, SARIF review/note. |
| #19 | Internal/external clauses share one process. Mixed-clause order witnesses use measured top-level selection and recorded seed; replay supports them, compile refuses with the command. | In-memory fixture repro → replay 3/3 → compile exit 5; same-clause fallback and seeded-selection tests. |
| #21 | Measured three production-global shapes: evidence missing in 3/3, confirmed witness found in 3/3. Optional deeper analysis remains deferred because this matrix shows no witness recall loss. | Direct read, direct write and method mutation fixtures pin both missing static evidence and successful dynamic reproduction. |
| #23 | `hunt --jobs N`, default min(NumCPU,4), schedules packages concurrently under one budget. Canonical deduplication and directory locks preserve package isolation; reports merge in input order. | Overlap, deduplication, jobs=1, report ordering, shared budget cap and partial capsule; all under race detection. |

Validation:

- `go test -race -count=1 ./...`
- `go vet ./...`
- `scripts/action-smoke.sh` (markdown, JSON, gate and artifact behavior)
- `go build ./...` and `go vet ./...` in all 14 fixture modules
- `git diff --check`

Only Action JSON counting changes in `action.yml`: it counts `status: confirmed`
instead of capsule IDs so suspected/partial artifacts cannot inflate the
confirmed count. The generated opt-in replay gate uses the installed executable
rather than `go run` so Go's wrapper cannot collapse exit codes to 1.

Limitations are deliberate: dependency hints are syntax-based and package-wide;
external services are not emulated. The global evidence matrix is a measurement,
not a claim of complete static analysis. The cross-clause parallel witness form
remains unsupported; #19 concerns order. Repositories with shared external state
between otherwise separate packages should select `--jobs 1`.
