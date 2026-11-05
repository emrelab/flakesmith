## What changed

One topic per PR — the commit message rule applies to the PR as a whole.

## Preflight (all three must be green locally)

- [ ] `make all` (fmt + vet + test -race + build)
- [ ] `make fixtures`
- [ ] `make smoke` (action scripts, no GitHub needed)

## Fixture matrix

If `testdata/fixtures/` changed, state the expected class and verdict for each
new/edited fixture here:

| Fixture | Expected class | `-parallel=1` | `-parallel>=2` |
|---|---|---|---|

## Notes for the reviewer

<!-- Why this is the minimal change that fixes the problem. -->
