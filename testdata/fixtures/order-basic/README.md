# fixture: order-basic

**Expected class:** `KindOrder` · polluter = `TestPolluter`

Verified behavior:

| Command | Result |
|---|---|
| `-run '^TestVictim$'` | PASS |
| `-run '^(TestPolluter\|TestVictim)$'` | FAIL (`seeded=42`) |
| generated `zz_flakesmith_witness_0_test.go` + `-parallel=1` | FAIL (5/5) |

The third row is the harness acceptance criterion and has been verified by
hand.
