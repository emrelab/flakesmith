package clausemismatch

// Inner test package: the test function here cannot be called from the outer package.
var shared int

func SetShared(v int) { shared = v }

func Shared() int { return shared }
