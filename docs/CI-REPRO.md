---
render_with_liquid: false
layout: default
title: CI reproduction recipe
---

# CI recipe: a red test log → witness → one PR comment

The pieces already ship; this page is the glue between them (#17).

```
red `go test -json` log  →  flakesmith repro --from-json  →  witness  →  one PR comment
```

The point is that the developer pastes nothing. CI already paid for the discovery
(the log names the failing tests); `repro` only has to shrink them.

**Rules the recipe keeps, because the tool's honesty depends on them:**

- **Post only on exit `0`.** Exit `3` means "did not reproduce within budget" and
  exit `4` means "budget exhausted" — in both cases there is no witness to show,
  and a comment saying "found a flake" would be a lie. Silence is the answer.
- **One comment, updated in place.** Without a marker every red build stacks
  another identical comment on the PR.
- **A log is not evidence.** The comment says which test, which polluter and the
  witness command; the capsule is the artifact.

---

## The workflow

Two jobs: the existing test job saves its `-json` log even when it fails, and a
follow-up job turns that log into a comment. Drop it into the workflow that
already runs `go test`.

```yaml
permissions:
  contents: read
  pull-requests: write

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: go test (keep the log even when red)
        # `tee` + `pipefail` keeps the failing exit code, so the job still goes red.
        run: |
          set -o pipefail
          go test -json ./... | tee "$RUNNER_TEMP/go-test.json"

      - name: save the -json log
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: go-test-json
          path: ${{ runner.temp }}/go-test.json
          if-no-files-found: ignore

  repro:
    needs: test
    if: needs.test.result == 'failure'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: actions/download-artifact@v4
        with:
          name: go-test-json
          path: ${{ runner.temp }}

      # Built from source on purpose: this job wants the same commit that just
      # failed, not a released binary.
      - name: install flakesmith
        run: go build -o "$RUNNER_TEMP/flakesmith" ./cmd/flakesmith

      - name: shrink the failures to a witness
        id: repro
        # `--format markdown` is already a PR-comment body. Budget is deliberately
        # smaller than a full hunt: this job has suspects, not a search space.
        run: |
          set -uo pipefail
          "$RUNNER_TEMP/flakesmith" repro \
            --from-json "$RUNNER_TEMP/go-test.json" \
            --format markdown --budget 200 --timeout 10m > "$RUNNER_TEMP/repro.md"
          echo "code=$?" >> "$GITHUB_OUTPUT"

      # Exit 0 = a confirmed witness. 3/4 = no witness, and no comment.
      - name: comment on the PR
        if: steps.repro.outputs.code == '0' && github.event_name == 'pull_request'
        env:
          GH_TOKEN: ${{ github.token }}
          PR: ${{ github.event.pull_request.number }}
        run: |
          set -uo pipefail
          printf '<!-- flakesmith-repro -->\n' | cat - "$RUNNER_TEMP/repro.md" > "$RUNNER_TEMP/body.md"
          printf '\n_Reproduced from the failing CI log; the capsule is attached to this run._\n' >> "$RUNNER_TEMP/body.md"
          gh pr comment "$PR" --body-file "$RUNNER_TEMP/body.md" --edit-last --create-if-none

      - name: keep the capsules
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: flakesmith-capsules
          path: "**/.flakesmith/*.json"
          if-no-files-found: ignore
```

## What was verified, and what was not

Verified locally on this repo (the command, its output shape and every exit code
below). The real output, indented because it contains a fence of its own:

    $ flakesmith repro --from-json failure.json --format markdown --budget 400 --timeout 120s
    ### flake: TestVictim
    
    | field | value |
    |---|---|
    | package | fixture/orderbasic |
    | class | order |
    | polluter | TestPolluter |
    | run | 3/3 failed |
    | evidence | TestPolluter writes, TestVictim reads: seeded |
    | status | confirmed |
    | capsule | `fs_ad2495a77e2c` |
    
    Reproduce:
    
    ```sh
    go test . -run '^TestFlakeWitness$' -count=1 -parallel=1
    ```
    
    Permanent regression test: `checkout_TestVictim_flakesmith_test.go` (build tag `flakesmith`).
    
- `--format markdown` is the comment body, byte for byte — no templating needed.
- `--from-json` resolves import paths with `go list`, so it must run inside the
  repository the log came from.
- Exit codes: `0` witness found, `3` not reproduced, `4` budget exhausted
  (`internal/cli/repro.go:77-83`).
- `gh pr comment --edit-last --create-if-none` (gh ≥ 2.45) is the upsert: it edits
  this job's previous comment when there is one and creates it otherwise, so a
  PR sees one witness block, not one per red build.

**Not** verified here: the workflow itself. It is a template — the `needs`/`if`
plumbing, artifact paths and the `gh` token scopes belong to your repository.
Test it on a branch with an intentionally red test before trusting it on `main`.

## Why this is opt-in

Posting to a PR is a write outside the repository. A tool that comments
unprompted gets uninstalled, and the soft-mode default of the packaged Action is
deliberate: findings annotate, they do not block, and they do not speak unless
asked.
