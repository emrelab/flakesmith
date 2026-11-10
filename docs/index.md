---
title: flakesmith
---

# flakesmith

Reproduce a flaky Go test in 30 seconds instead of 30 minutes, shrink it to the
smallest polluter→victim witness, and leave a permanent regression test behind.

```sh
go install github.com/emrelab/flakesmith/cmd/flakesmith@v0.2.0
```

- [Install & first 5 minutes](https://github.com/emrelab/flakesmith#install)
- [Architecture](ARCHITECTURE.md) — package boundaries, algorithms, Go traps
- [CI reproduction recipe](CI-REPRO.md) — turn a red `go test -json` log into a PR comment
- [Real-world evidence](real-world/README.md) — two upstream flake cases, witness before/after
- [Autonomous runbook](AUTO.md)

Apache-2.0. Not affiliated with any hosted vendor.
