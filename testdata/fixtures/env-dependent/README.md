# fixture: env-dependent

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own: `cd testdata/fixtures/env-dependent && go build ./...`

**Expected class:** `KindEnv` — the key `FIXTURE_MODE` is stored in the
capsule, the value never is.
