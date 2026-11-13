// Command go-flake-hunter reproduces flakes in Go tests and generates a
// regression test that can be committed to the repository.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/emrelab/go-flake-hunter/internal/cli"
)

// version is filled at build time via -ldflags "-X main.version=0.1.0".
var version = "dev"

func main() {
	cli.Version = version

	// Ctrl-C / SIGTERM: forwarded to running go test subprocesses and to
	// temporary harness cleanup. main only sets up the context here.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Execute(ctx, os.Args[1:], cli.IO{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
}
