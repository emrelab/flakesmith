package report

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Text writes the human-readable summary of a found witness.
func Text(w io.Writer, c domain.Capsule) error {
	v := c.Witness
	var b strings.Builder

	fmt.Fprintf(&b, "\nflake found: %s\n", v.Victim.Name)
	fmt.Fprintf(&b, "  package      : %s\n", v.Victim.Package)
	fmt.Fprintf(&b, "  class        : %s\n", v.Kind)

	if len(v.Polluters) > 0 {
		names := make([]string, len(v.Polluters))
		for i, p := range v.Polluters {
			names[i] = p.Name
		}
		fmt.Fprintf(&b, "  polluter     : %s\n", strings.Join(names, ", "))
	}
	if len(v.EnvDiff) > 0 {
		fmt.Fprintf(&b, "  environment  : %s\n", strings.Join(v.EnvDiff, ", "))
	}

	fmt.Fprintf(&b, "  run          : %d/%d failed\n", c.Observation.Failures, c.Observation.Runs)
	if c.Observation.Deterministic {
		b.WriteString("  deterministic: yes\n")
	} else {
		b.WriteString("  deterministic: no (probabilistic)\n")
	}

	if g := sharedGlobalLine(c); g != "" {
		fmt.Fprintf(&b, "  evidence     : %s\n", g)
	}
	if len(c.Evidence.FilesTouched) > 0 {
		fmt.Fprintf(&b, "  files        : %s\n", strings.Join(c.Evidence.FilesTouched, "; "))
	}

	fmt.Fprintf(&b, "\n  reproduce:\n    %s\n", c.Repro.Command)
	fmt.Fprintf(&b, "\n  capsule      : %s\n", c.ID)
	fmt.Fprintf(&b, "  status       : %s\n", c.Status)
	// Note: the witness was verified but the evidence is weak at one point
	// (e.g. it did not reproduce in a --race run). One line, this is what the user reads.
	if c.Note != "" {
		fmt.Fprintf(&b, "  note         : %s\n", c.Note)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// sharedGlobalLine turns the static evidence into the single most useful
// sentence in the report: a package var one test writes and another reads.
// It is phrased as an observation, never as a proven root cause.
func sharedGlobalLine(c domain.Capsule) string {
	written := map[string]string{}
	for test, vars := range c.Evidence.GlobalsWritten {
		for _, v := range vars {
			written[v] = test
		}
	}
	var lines []string
	for test, vars := range c.Evidence.GlobalsRead {
		for _, v := range vars {
			if writer, ok := written[v]; ok && writer != test {
				lines = append(lines, fmt.Sprintf("%s writes, %s reads: %s", writer, test, v))
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	sortStrings(lines)
	return lines[0]
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// NoFlake explains an empty hunt honestly: which hypotheses ran and what each
// said. A bare "not found" hides the difference between "nothing wrong" and
// "the probe could not see this class of bug".
func NoFlake(w io.Writer, attempts []AttemptInfo) error {
	var b strings.Builder
	b.WriteString("\nflake not found.\n")
	if len(attempts) == 0 {
		b.WriteString("  (no probe ran)\n")
	} else {
		b.WriteString("  tried:\n")
		for _, a := range attempts {
			fmt.Fprintf(&b, "    - %-9s %s\n", a.Probe, attemptReason(a))
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// AttemptInfo is the report-facing shape of probe.Attempt (avoids importing
// probe from report, keeping the dependency direction clean).
type AttemptInfo struct {
	Probe         string
	Err           error
	NotReproduced bool
	BuildFailed   bool
	// Skipped is set when the probe ruled itself out from static facts alone:
	// no build was spent, and Reason says why. Reporting it as "does not explain"
	// would claim we searched where we did not.
	Skipped bool
	Reason  string
}

// attemptReason is the one-line verdict of a probe, shared by every renderer so
// "tried" cannot read differently in text and in partial reports.
func attemptReason(a AttemptInfo) string {
	switch {
	case a.Skipped:
		if a.Reason != "" {
			return "not applicable — " + a.Reason
		}
		return "not applicable"
	case a.Err == nil:
		return "witness found"
	case a.BuildFailed:
		return "package does not build"
	case errors.Is(a.Err, context.Canceled), errors.Is(a.Err, context.DeadlineExceeded):
		return "cancelled"
	case errors.Is(a.Err, domain.ErrBudgetExceeded):
		return strings.TrimSuffix(a.Err.Error(), ": "+domain.ErrBudgetExceeded.Error())
	case a.NotReproduced:
		return "does not explain"
	default:
		return a.Err.Error()
	}
}

// Partial writes the report for a hunt that ended without a witness because it
// ran out of budget or was interrupted.
//
// A partial capsule is not a failure to report an error; it is the honest
// record of what was ruled out so far, and it is worth more than an empty
// answer: the next run can start from the probes already known to say nothing.
func Partial(w io.Writer, c domain.Capsule, attempts []AttemptInfo, path, hint string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "\nno result: %s (%s)\n", c.Target.Victim, c.Note)
	fmt.Fprintf(&b, "  package      : %s\n", c.Target.Package)
	if len(attempts) > 0 {
		b.WriteString("  tried:\n")
		for _, a := range attempts {
			fmt.Fprintf(&b, "    - %-9s %s\n", a.Probe, attemptReason(a))
		}
	} else {
		b.WriteString("  (no probe completed)\n")
	}
	fmt.Fprintf(&b, "\n  partial capsule: %s\n", path)
	fmt.Fprintf(&b, "  status         : %s\n", c.Status)
	if hint != "" {
		fmt.Fprintf(&b, "  %s\n", hint)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
