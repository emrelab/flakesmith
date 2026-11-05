package gotest

import (
	"errors"
	"os/exec"
	"strings"
)

// asExitError reports whether err is an *exec.ExitError (a non-zero exit,
// which for `go test` means failing tests — the normal path).
func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}

// firstLines returns the first n non-empty lines, joined.
func firstLines(s string, n int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(line))
		if len(out) == n {
			break
		}
	}
	return strings.Join(out, " | ")
}

// tail returns the last n non-empty lines, joined.
func tail(s string, n int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(line))
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return strings.Join(out, " | ")
}
