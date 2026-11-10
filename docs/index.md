---
layout: default
title: flakesmith
description: Reproduce, shrink and permanently capture flaky Go tests.
---

Reproduce a flaky Go test in 30 seconds instead of 30 minutes, shrink it to the
smallest polluter→victim witness, and leave a permanent regression test behind.

```sh
go install github.com/emrelab/flakesmith/cmd/flakesmith@v0.2.0
```

| Where to go | What is there |
|---|---|
| [GitHub repository](https://github.com/emrelab/flakesmith) | install, the first 5 minutes, every command and exit code |
| [Architecture](ARCHITECTURE.md) | package boundaries, algorithms, Go-specific traps |
| [CI reproduction recipe](CI-REPRO.md) | turn a red `go test -json` log into a PR comment |
| [Real-world evidence](real-world/README.md) | two upstream flake cases, witness before and after |
| [Autonomous runbook](AUTO.md) | the scheduled loop and its rules |

Apache-2.0. Not affiliated with any hosted vendor.
