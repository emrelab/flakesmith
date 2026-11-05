package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Markdown renders the capsule as GitHub-flavoured markdown.
//
// Reason to exist: capsules must read well in CI logs and PR comments. Plain
// text wraps lines out of shape and JSON is unreadable; the table is the only
// format between them — it renders on GitHub and looks honest in a terminal.
func Markdown(w io.Writer, c domain.Capsule) error {
	var b strings.Builder
	v := c.Witness

	title := c.Target.Victim
	if c.Status != domain.StatusConfirmed {
		title = fmt.Sprintf("%s (%s)", title, c.Status)
	}
	fmt.Fprintf(&b, "### flake: %s\n\n", title)

	b.WriteString("| field | value |\n|---|---|\n")
	row := func(k, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(&b, "| %s | %s |\n", k, escapePipe(val))
	}
	row("package", c.Target.Package)
	row("class", string(c.Kind))
	if len(v.Polluters) > 0 {
		names := make([]string, len(v.Polluters))
		for i, p := range v.Polluters {
			names[i] = p.Name
		}
		row("polluter", strings.Join(names, ", "))
	}
	if len(v.EnvDiff) > 0 {
		row("environment key", strings.Join(v.EnvDiff, ", "))
	}
	if c.Observation.Runs > 0 {
		row("run", fmt.Sprintf("%d/%d failed", c.Observation.Failures, c.Observation.Runs))
	}
	row("evidence", sharedGlobalLine(c))
	if len(c.Evidence.FilesTouched) > 0 {
		row("disk writes", strings.Join(c.Evidence.FilesTouched, "; "))
	}
	row("status", string(c.Status))
	row("capsule", "`"+c.ID+"`")
	if c.Note != "" {
		row("note", c.Note)
	}

	if c.Repro.Command != "" {
		fmt.Fprintf(&b, "\nReproduce:\n\n```sh\n%s\n```\n", c.Repro.Command)
	}
	if c.Repro.GeneratedTest != "" {
		fmt.Fprintf(&b, "\nPermanent regression test: `%s` (build tag `%s`).\n",
			c.Repro.GeneratedTest, domain.PersistentBuildTag)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// escapePipe keeps a value from breaking the table it lives in.
func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
