package policy_test

import (
	"strings"
	"testing"

	"github.com/emrelab/flakesmith/internal/policy"
)

// The value must never leak into output under any circumstance: the capsule is committed to the repo.
func TestRedactEnvNeverLeaksValue(t *testing.T) {
	secret := "ghp_1234567890abcdef"
	got := policy.RedactEnv("GITHUB_TOKEN=" + secret)
	if strings.Contains(got, secret) {
		t.Fatalf("secret leaked: %q", got)
	}
	if got != "GITHUB_TOKEN="+policy.RedactionPlaceholder {
		t.Errorf("broken format: %q", got)
	}
}

// A valueless input must stay exactly as it is: "key present, value redacted"
// and "key entirely absent" must not be confused.
func TestRedactEnvWithoutValue(t *testing.T) {
	if got := policy.RedactEnv("MODE"); got != "MODE" {
		t.Errorf("valueless input changed: %q", got)
	}
}

// An empty value returns "", not the placeholder: absence must be reportable.
func TestRedactValueEmptyStaysEmpty(t *testing.T) {
	if got := policy.RedactValue(""); got != "" {
		t.Errorf("empty value = %q", got)
	}
	if got := policy.RedactValue("ci"); got != policy.RedactionPlaceholder {
		t.Errorf("non-empty value was not redacted: %q", got)
	}
}
