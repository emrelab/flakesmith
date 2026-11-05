# fixture: file-leak

**Expected class:** `KindOrder` + populated `Evidence.FilesTouched` · `HasTestMain = true`

The polluter writes to a fixed path (it does not use `t.TempDir`), the victim
expects the file to be absent. `TestMain` deletes the file at startup.

This fixture also covers the `facts.HasTestMain` path and represents external
(filesystem) pollution — the order-dependent failure mode that lives outside
the test process.
