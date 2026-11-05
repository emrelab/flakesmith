package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/emrelab/flakesmith/internal/domain"
)

// workflowPath is where the generated CI job lives.
const workflowPath = ".github/workflows/flakesmith.yml"

// workflowTemplate is the job `init` writes.
//
// Deliberately narrow: it hints (`continue-on-error`) instead of gating, because
// a newly adopted flake hunter that turns CI red on day one gets removed on day
// two. `--fail-on-flake` is one line away in the file for teams that want a gate.
const workflowTemplate = `# flakesmith: flaky test hunting.
#
# Generated: flakesmith init. Edit freely; this file is never
# written again (init does not touch an existing file).
name: flakesmith

on:
  pull_request:
  workflow_dispatch:
  schedule:
    - cron: "0 3 * * 1" # weekly: so new flakes do not pile up silently

jobs:
  hunt:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      # Note: soft mode on first adoption so CI is not broken. For a gate,
      # add --fail-on-flake and delete continue-on-error.
      - name: hunt flakes
        continue-on-error: true
        run: |
          set -o pipefail
          go run github.com/emrelab/flakesmith/cmd/flakesmith@latest hunt ./... \
            --format markdown --budget 200 --timeout 10m | tee flakesmith-report.md

      - name: attach report
        if: always()
        run: cat flakesmith-report.md >> "$GITHUB_STEP_SUMMARY"

      - name: store capsules
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: flakesmith-capsules
          path: "**/.flakesmith/*.json"
          if-no-files-found: ignore
`

// initCmd writes the CI scaffolding a repository needs.
//
// It refuses to overwrite: a team that edited the workflow (added a matrix,
// changed the gate) must not lose that on a second run. Re-running is therefore
// safe and idempotent.
func initCmd(ctx context.Context, args []string, out IO) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(out.Stderr)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	dir := "."
	switch rest := fs.Args(); {
	case len(rest) > 1:
		return domain.Wrap(domain.ErrUsage, "at most one directory")
	case len(rest) == 1:
		dir = rest[0]
	}

	path := filepath.Join(dir, workflowPath)
	if _, err := os.Stat(path); err == nil {
		_, _ = fmt.Fprintf(out.Stdout, "already exists, untouched: %s\n", path)
		return nil
	} else if !errors.Is(err, iofs.ErrNotExist) {
		return domain.Wrap(domain.ErrUsage, "could not read %s: %v", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.Wrap(domain.ErrInternal, "could not create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(workflowTemplate), 0o644); err != nil {
		return domain.Wrap(domain.ErrInternal, "could not write %s: %v", path, err)
	}

	_, _ = fmt.Fprintf(out.Stdout, "written: %s\n", path)
	_, _ = fmt.Fprintln(out.Stdout, "\nNext step: flakesmith hunt ./... (run it once locally, see the output)")
	return nil
}
