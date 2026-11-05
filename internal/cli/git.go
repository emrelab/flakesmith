package cli

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// gitOutput runs one git command in dir and returns its trimmed stdout.
//
// A separate helper: --changed asks git twice (does the ref exist, what is
// the diff), and swallowing stderr on every call would be the easiest way to
// make a missing ref look like "no changes" — errors are returned.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", err
		}
		return "", &gitError{err: err, msg: msg}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// gitError keeps git's own sentence: "fatal: bad revision" is the diagnosis the
// user needs, not the exit status.
type gitError struct {
	err error
	msg string
}

func (e *gitError) Error() string { return e.msg }
func (e *gitError) Unwrap() error { return e.err }

// gitState returns the short commit and whether the worktree is dirty.
//
// Best-effort on purpose: a capsule outside a git repo is still useful, so a
// missing git must not fail the hunt. It only weakens the id's stability.
func gitState(dir string) (string, bool) {
	commit := "unknown"
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			commit = s
		}
	}
	dirty := false
	if out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output(); err == nil {
		dirty = strings.TrimSpace(string(out)) != ""
	}
	return commit, dirty
}
