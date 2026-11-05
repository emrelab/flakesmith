# fixture: clean

**Expected:** no class matches → `flake not found.` (with the hypothesis list), exit code **0**.

The reference point: two independent tests (no shared globals, no files, no
env reads, no `t.Parallel`). Every other fixture deviates from here by exactly
one thing.

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module.
