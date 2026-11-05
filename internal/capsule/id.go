package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
)

// ID derives the content-addressed identity of a witness.
//
// Everything that changes WHEN or WHERE the flake appears is excluded
// (timestamp, host, run count) so that the same defect keeps the same id:
// that is the difference between "a capsule per run" and "a capsule per bug".
func ID(w domain.Witness, module, commit string) string {
	parts := []string{
		domain.SchemaVersion,
		module,
		commit,
		w.Victim.Package,
		w.Victim.Name,
		string(w.Kind),
	}
	polluters := make([]string, len(w.Polluters))
	for i, p := range w.Polluters {
		polluters[i] = p.Name
	}
	sort.Strings(polluters)
	parts = append(parts, polluters...)

	parts = append(parts,
		fmt.Sprintf("parallel=%d", w.Flags.Parallel),
		fmt.Sprintf("race=%t", w.Flags.Race),
	)

	env := append([]string(nil), w.EnvDiff...)
	sort.Strings(env)
	parts = append(parts, env...)

	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "fs_" + hex.EncodeToString(sum[:])[:12]
}

// PartialID names a capsule that has no verified witness: the hunt stopped
// (budget, cancel, unsupported class) before one existed.
//
// Status and reason are part of the identity because they are different
// findings, not the same bug seen twice: "budget exhausted" and "cancelled"
// must not overwrite each other. Timestamps stay out, so re-running a
// budget-limited hunt over the same code rewrites one file instead of
// accumulating one per attempt.
func PartialID(target domain.Target, status domain.CapsuleStatus, reason, module, commit string) string {
	parts := []string{
		domain.SchemaVersion,
		module,
		commit,
		target.Package,
		target.Victim,
		string(status),
		reason,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "fs_" + hex.EncodeToString(sum[:])[:12]
}
