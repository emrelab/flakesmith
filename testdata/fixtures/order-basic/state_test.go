package orderbasic

// This fixture is the barest form of order dependence:
// TestPolluter pollutes the package variable, TestVictim expects it clean.
// Expected: KindOrder, polluter = TestPolluter.

var seeded int

func reset() { seeded = 0 }
