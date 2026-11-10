# Cross-clause order fixture

`clausemismatch` and `clausemismatch_test` are linked into one test binary.
The inner test writes an in-memory global and the outer test observes it.
The victim passes alone and fails after the polluter in a single process.

Expected: hunt/repro records a confirmed `crossClause` witness; replay selects
both top-level tests in one invocation. Compile refuses with exit 5 and gives
the recorded command, because one generated Go file cannot call both clauses.
