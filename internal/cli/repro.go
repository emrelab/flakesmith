package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/emrelab/flakesmith/internal/domain"
	"github.com/emrelab/flakesmith/internal/gotest"
)

// reproCmd re-derives and shrinks the witness for one known test.
//
// Difference from hunt: hunt asks "is anything here flaky?" and pays for a full
// scan; repro is handed the suspect (the name usually comes from a previous
// capsule or a CI failure) and spends its whole budget on that one test. Same
// scanner underneath, so the classification cannot drift between the commands.
func reproCmd(ctx context.Context, args []string, out IO) error {
	var o huntOptions
	fs := flag.NewFlagSet("repro", flag.ContinueOnError)
	fs.SetOutput(out.Stderr)
	o.addFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 || len(rest) > 2 {
		_, _ = fmt.Fprint(out.Stderr, "usage: flakesmith repro <package-dir> [TestName]\n")
		return domain.ErrUsage
	}
	pkg := rest[0]
	var only string
	if len(rest) == 2 {
		only = rest[1]
	}

	dirs, err := gotest.ExpandPatterns(ctx, []string{pkg})
	if err != nil {
		return err
	}
	s := newScanner(out, o, only)
	var runErr error
	for _, dir := range dirs {
		if err := s.pkg(ctx, dir); err != nil {
			runErr = err
			break
		}
	}
	if err := s.flush(); err != nil {
		return err
	}
	if runErr != nil {
		return runErr
	}
	return s.outcome(true)
}
