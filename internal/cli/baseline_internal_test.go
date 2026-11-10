package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/capsule"
	"github.com/emrelab/flakesmith/internal/domain"
)

func TestBaselineWriteMigrationAndPreservation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    bool
	}{
		{"legacy", `{"schema":"flakesmith/baseline/v1","ids":["fs_old"]}`, false},
		{"current", `{"schema":"flakesmith/baseline/v2","ids":["fd_known"]}`, false},
		{"corrupt", `{broken`, true},
		{"unknown", `{"schema":"future","ids":[]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "baseline.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			err := writeBaselineFile(IO{Stderr: &stderr}, &scanner{ids: []string{"fd_new"}}, path)
			if tc.wantErr {
				after, _ := os.ReadFile(path)
				if !errors.Is(err, domain.ErrUsage) || string(after) != tc.body {
					t.Fatalf("invalid baseline not preserved: %v %s", err, after)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			b, err := capsule.LoadBaseline(path)
			if err != nil || !b.Contains("fd_new") {
				t.Fatalf("new finding lost: %v %v", b, err)
			}
			if tc.name == "current" && !b.Contains("fd_known") {
				t.Fatal("existing fingerprint lost")
			}
			if tc.name == "legacy" && (b.Contains("fs_old") || !strings.Contains(stderr.String(), "rescan every")) {
				t.Fatalf("legacy migration was silent or mixed hashes: %v %s", b, stderr.String())
			}
		})
	}
}
