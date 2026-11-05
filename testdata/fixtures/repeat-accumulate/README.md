# fixture: repeat-accumulate

This module lives under `testdata/fixtures`; the Go tool excludes it from the
root module. It builds on its own: `cd testdata/fixtures/repeat-accumulate && go build ./...`

**Expected class:** `KindRepeat` — shared state carried across `-count`
rounds within one process (`TestAccumulates`).
