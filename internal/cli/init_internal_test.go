package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReplayScaffoldGatePreservesExitContract(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "flakesmith-bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for path, script := range map[string]string{filepath.Join(bin, "flakesmith"): "#!/bin/sh\nexit \"$REPLAY_STATUS\"\n", filepath.Join(bin, "go"): "#!/bin/sh\nexit 0\n"} {
		if err := os.WriteFile(path, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	gate := replayGate(".flakesmith/fs_123.json")
	_, body, ok := strings.Cut(gate, "        run: |\n")
	if !ok {
		t.Fatal("no run block")
	}
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	script := strings.Join(lines, "\n")
	for input, want := range map[int]int{0: 1, 3: 0, 6: 6, 130: 130} {
		cmd := exec.Command("bash", "-c", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "RUNNER_TEMP="+filepath.ToSlash(dir), "REPLAY_STATUS="+strconv.Itoa(input))
		output, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			e, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			got = e.ExitCode()
		}
		if got != want {
			t.Fatalf("replay %d -> gate %d, want %d: %s", input, got, want, output)
		}
	}
}
