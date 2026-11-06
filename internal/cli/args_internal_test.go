package cli

import (
	"flag"
	"io"
	"testing"
)

// reorderFlags must ask the FlagSet which flags are boolean, never a static
// list: every new bool flag used to be a silent misparse trap (`hunt ./...
// --short --budget 200` gave "200" to -short and --budget faced --timeout —
// the usage error that killed self-hunt). These cases pin both directions:
// bools do not swallow, value flags do, and the combination parses.
func TestReorderFlagsKnowsItsBools(t *testing.T) {
	fs := flag.NewFlagSet("hunt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	short := fs.Bool("short", false, "")
	budget := fs.Int("budget", 0, "")
	format := fs.String("format", "text", "")

	args := reorderFlags(fs, []string{"./...", "--short", "--budget", "200", "--format", "json", "./pkg"})
	if err := fs.Parse(args); err != nil {
		t.Fatalf("reordered parse: %v (args=%v)", err, args)
	}
	if !*short || *budget != 200 || *format != "json" {
		t.Errorf("short=%v budget=%d format=%q", *short, *budget, *format)
	}
	if len(fs.Args()) != 2 || fs.Args()[0] != "./..." || fs.Args()[1] != "./pkg" {
		t.Errorf("positional args = %v, want [./... ./pkg]", fs.Args())
	}
}
