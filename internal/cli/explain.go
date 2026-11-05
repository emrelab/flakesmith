package cli

import (
	"context"
	"flag"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/domain"
)

// explainCmd renders a stored capsule for a human.
//
// Hard rule: it never runs a test. Its job is to answer "what did we find, and
// why should I believe it?" on a machine where `go` is not even installed —
// that is what makes a capsule useful in a PR comment or a bug report.
func explainCmd(ctx context.Context, args []string, out IO) error {
	var (
		format string
		dir    string
	)
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(out.Stderr)
	fs.StringVar(&format, "format", "text", "output format: text|json|markdown")
	fs.StringVar(&dir, "capsule-dir", "", "capsule directory (default .flakesmith)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	id, storeDir, _, err := resolveCapsule(fs.Args(), dir, "")
	if err != nil {
		return err
	}
	c, err := capsule.NewStore(storeDir).Read(id)
	if err != nil {
		return domain.Wrap(domain.ErrUsage, "%s", err)
	}
	return writeCapsule(out, format, c)
}
