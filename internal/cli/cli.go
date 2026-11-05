// Package cli manages the command-line surface and the exit codes.
//
// Execute is pure: stdout/stderr are injected, it never calls os.Exit.
// So every command can be verified with a table test.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Version is filled at build time via -ldflags.
var Version = "dev"

// IO are the streams the command writes to.
type IO struct {
	Stdout io.Writer
	Stderr io.Writer
	// Debug is enabled by --debug: internal error detail is shown to the user.
	// In normal mode only an id and "use --debug for detail" are printed.
	Debug bool
}

// Command is a single subcommand.
type Command struct {
	Name    string
	Summary string
	// Task is the development task that introduced the command
	// (docs/ARCHITECTURE.md §12). 0 -> shipped in this release.
	Task int
	Run  func(ctx context.Context, args []string, out IO) error
}

// Execute routes arguments to a command and returns the exit code.
func Execute(ctx context.Context, args []string, out IO) int {
	if out.Stdout == nil {
		out.Stdout = io.Discard
	}
	if out.Stderr == nil {
		out.Stderr = io.Discard
	}

	// --debug is global: extracted here instead of adding it to every
	// command's flag set, so `hunt . --debug` works too.
	args, out.Debug = stripDebug(args)

	cmds := commands()
	if len(args) == 0 {
		usage(out.Stderr, cmds)
		return domain.ExitUsage
	}

	name := args[0]
	switch name {
	case "-h", "--help", "help":
		usage(out.Stdout, cmds)
		return domain.ExitOK
	case "-v", "--version", "version":
		_, _ = fmt.Fprintf(out.Stdout, "flakesmith %s\n", Version)
		return domain.ExitOK
	}

	for _, c := range cmds {
		if c.Name != name {
			continue
		}
		if c.Run == nil {
			_, _ = fmt.Fprintf(out.Stderr, "flakesmith: %q not implemented yet (task %d)\n", c.Name, c.Task)
			return domain.ExitUsage
		}
		err := c.Run(ctx, args[1:], out)
		if err == nil {
			return domain.ExitOK
		}
		// -h/--help: the flag package already printed usage. A help request
		// is not a usage error; ExitCode(err) would give this a 2 and mislead
		// CI into thinking the command is broken.
		if errors.Is(err, flag.ErrHelp) {
			return domain.ExitOK
		}
		if msg := message(err, out.Debug); msg != "" {
			_, _ = fmt.Fprintf(out.Stderr, "flakesmith: %s\n", msg)
		}
		return domain.ExitCode(err)
	}

	_, _ = fmt.Fprintf(out.Stderr, "flakesmith: unknown command %q\n\n", name)
	usage(out.Stderr, cmds)
	return domain.ExitUsage
}

// parseFlags is the single entry point for flag parsing. flag.ErrHelp is
// returned as-is — a help request is not a usage error and maps to ExitOK
// in Execute. Folding everything into ErrUsage would report `hunt --help`
// to CI as exit 2, "the command is broken".
func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(reorderFlags(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return domain.ErrUsage
	}
	return nil
}

// stripDebug removes the global --debug flag from the argument list.
func stripDebug(args []string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	debug := false
	for _, a := range args {
		if a == "--debug" {
			debug = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, debug
}

// message picks the text shown to the user.
// For ErrInternal the detail shows only with --debug; a stack is never shown.
//
// # Wrapped errors carry their cause as a suffix
//
// Wrapped errors carry their cause as a suffix ("...: unsupported test
// format"). ExitCode already turned that cause into the exit code, so repeating
// it in the message adds nothing a user can act on — the sentence before it is
// the diagnosis.
func message(err error, debug bool) string {
	var ie internalError
	if errors.As(err, &ie) {
		if debug && ie.err != nil {
			return "internal error: " + ie.id + ": " + ie.err.Error()
		}
		return "internal error: " + ie.id + " (use --debug for detail)"
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled (SIGINT); unfinished witness files deleted"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout exceeded; increase it with --timeout"
	}
	return trimCause(err.Error())
}

func trimCause(s string) string {
	for _, cause := range []error{
		domain.ErrNotReproduced, domain.ErrNotReproducible, domain.ErrUnsupportedTest,
		domain.ErrBudgetExceeded, domain.ErrBuildFailed, domain.ErrUsage,
		domain.ErrFlakeFound, domain.ErrInternal,
	} {
		s = strings.TrimSuffix(s, ": "+cause.Error())
	}
	return s
}

// internalError hides unexpected errors from the user.
type internalError struct {
	id  string
	err error
}

func (e internalError) Error() string { return e.err.Error() }
func (e internalError) Unwrap() error { return e.err }

// Internal wraps an unexpected error without leaking it to the user.
func Internal(id string, err error) error { return internalError{id: id, err: err} }

func usage(w io.Writer, cmds []Command) {
	_, _ = fmt.Fprint(w, "flakesmith — flake reproduction and regression test generation for Go tests\n\n")
	_, _ = fmt.Fprint(w, "Usage:\n  flakesmith <command> [flags]\n\nGlobal flag: --debug (internal error detail)\n\nCommands:\n")
	sorted := make([]Command, len(cmds))
	copy(sorted, cmds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	width := 0
	for _, c := range sorted {
		width = max(width, len(c.Name))
	}
	for _, c := range sorted {
		state := ""
		if c.Run == nil {
			state = fmt.Sprintf(" [task %d]", c.Task)
		}
		_, _ = fmt.Fprintf(w, "  %-*s  %s%s\n", width, c.Name, c.Summary, state)
	}
	_, _ = fmt.Fprint(w, "\nDocs: docs/ARCHITECTURE.md\n")
}

// commands are the registered commands. Order is not shown to the user (printed alphabetically).
func commands() []Command {
	return []Command{
		{Name: "hunt", Summary: "scan packages, find flake candidates", Run: hunt},
		{Name: "repro", Summary: "reproduce and shrink one known flake", Run: reproCmd},
		{Name: "compile", Summary: "generate a permanent regression test from a capsule", Run: compileCmd},
		{Name: "replay", Summary: "run a capsule N times, verify determinism", Run: replayCmd},
		{Name: "explain", Summary: "render a capsule as a human-readable report", Run: explainCmd},
		{Name: "init", Summary: "write a GitHub Actions workflow scaffold", Run: initCmd},
	}
}
