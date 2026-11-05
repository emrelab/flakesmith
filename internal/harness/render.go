package harness

import (
	"bytes"
	"fmt"
	"go/format"
	"strconv"
	"text/template"

	"github.com/emrelab/flakesmith/internal/domain"
)

type templateData struct {
	Clause string
	Func   string
	Steps  []domain.Step
}

// Render produces gofmt-clean source for a witness file.
//
// format.Source runs BEFORE the bytes ever reach the filesystem: a witness we
// cannot compile is our bug, and it must never land in the user's worktree
// (a broken file would poison the next `go build` and look like their fault).
func Render(clause, fn string, steps []domain.Step) ([]byte, error) {
	return render(witnessTemplate, clause, fn, steps)
}

// RenderParallel renders a witness whose steps run as parallel subtests.
func RenderParallel(clause, fn string, steps []domain.Step) ([]byte, error) {
	return render(parallelTemplate, clause, fn, steps)
}

func render(tmplText, clause, fn string, steps []domain.Step) ([]byte, error) {
	if clause == "" {
		return nil, fmt.Errorf("%w: package clause unknown", domain.ErrInternal)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("%w: no witness steps", domain.ErrInternal)
	}

	// tmplText parses the selected template. This line was once pinned to
	// witnessTemplate and RenderParallel silently produced a serial file: the
	// witness built for the concurrency class never called t.Parallel(), so a
	// concurrency flake was never reproduced.
	tmpl, err := template.New("witness").Funcs(template.FuncMap{
		"quote": strconv.Quote,
	}).Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("%w: template: %v", domain.ErrInternal, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData{Clause: clause, Func: fn, Steps: steps}); err != nil {
		return nil, fmt.Errorf("%w: template execution: %v", domain.ErrInternal, err)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%w: generated witness syntax is invalid: %v", domain.ErrInternal, err)
	}
	return src, nil
}
