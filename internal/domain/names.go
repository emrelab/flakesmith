package domain

import "strings"

// Every generated file and function name is defined here (single source of truth).
//
// Critical: files starting with "_" or "." are SKIPPED by the go tool
// (verified on November 1, 2026), so the temporary harness takes a "zz_" prefix.
const (
	WitnessFilePrefix    = "zz_flakesmith_witness_"
	WitnessFileSuffix    = "_test.go"
	WitnessFuncName      = "TestFlakeWitness"
	PersistentFileSuffix = "_flakesmith_test.go"
	PersistentBuildTag   = "flakesmith"
	PersistentFuncPrefix = "TestFlakeRegression_"
	CapsuleDir           = ".flakesmith"
)

// WitnessFileName is the temporary harness file name. The suffix prevents collisions (1 per package).
func WitnessFileName(suffix string) string {
	if suffix == "" {
		suffix = "0"
	}
	return WitnessFilePrefix + suffix + WitnessFileSuffix
}

// IsWitnessFileName is used in stale-file cleanup.
func IsWitnessFileName(name string) bool {
	return strings.HasPrefix(name, WitnessFilePrefix) && strings.HasSuffix(name, WitnessFileSuffix)
}

// PersistentFileName is the persistent regression test file.
// A victim suffix prevents two tests in one source file overwriting each other.
// Omitting victim returns the legacy path for upgrade compatibility.
func PersistentFileName(sourceFile string, victim ...string) string {
	base := strings.TrimSuffix(sourceFile, "_test.go")
	base = strings.TrimSuffix(base, ".go")
	if len(victim) > 0 && victim[0] != "" {
		base += "_" + victim[0]
	}
	return base + PersistentFileSuffix
}

// PersistentFuncName is the function name of the generated regression test.
func PersistentFuncName(victim string) string { return PersistentFuncPrefix + victim }

// Subtest labels. Because the witness is a t.Run chain, the victim's result
// appears not in the top-level test but in the subtest carrying these labels:
//
//	--- FAIL: TestFlakeWitness/victim/TestVictim
//
// The predicate MUST search for this name; searching for "TestVictim" always
// returns skip and the search silently concludes "no flake".
const (
	StepLabelPolluter = "polluter/"
	StepLabelVictim   = "victim/"
)

// PolluterLabel is the subtest label of the polluter step.
func PolluterLabel(test string) string { return StepLabelPolluter + test }

// VictimLabel is the subtest label of the victim step.
func VictimLabel(test string) string { return StepLabelVictim + test }

// WitnessSubtest is the full subtest name to look for in a run result.
func WitnessSubtest(fn, test string) string { return fn + "/" + VictimLabel(test) }
