# cross-package

Issue #6 fixture: the polluter lives in package `a`, the victim in package
`b` — separate `go test` binaries. The pollution is a file, the only channel
that survives the process boundary. Hunt with:

    go-flake-hunter hunt <dir>/b --polluter-pkg <dir>/a
