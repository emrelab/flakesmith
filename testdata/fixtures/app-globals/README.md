# Application global evidence matrix (#21)

Three packages exercise a global declared in production code: direct write,
direct read, and mutation through a method. Tests intentionally leak state.
The default AST scan records no globals for these cases; their capsules have
no globalsWritten/globalsRead evidence. `repro` still finds a confirmed order
witness for all three because candidate selection follows observed test order,
not static global references. Tests pin both facts: evidence loss 3/3, witness
recall loss 0/3. This small matrix does not establish recall for arbitrary apps.
An optional deeper analysis is deferred until a measured recall benefit
justifies the extra analysis cost; no dependency or default scope is added.
