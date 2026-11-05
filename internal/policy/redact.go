package policy

// Redaction.
//
// Env values are never written to the capsule, only the key name. The reason is
// simple: the capsule is committed to the repository, and an env variable's value
// is most often a token, a DSN or a customer identity. A leak cannot be undone; a
// missing value is solved by simply telling the user "provide this key yourself".
//
// The key name is kept: without it the flake cannot be reproduced, and a key
// name on its own is not a secret (it is already written out in the code).

// RedactionPlaceholder is what a report shows instead of a value.
const RedactionPlaceholder = "<value redacted>"

// RedactValue is used by anything that renders an env value for a human.
//
// It returns the placeholder for a non-empty value and "" for an empty one, so
// "variable absent" and "variable present but value redacted" never mix.
func RedactValue(v string) string {
	if v == "" {
		return ""
	}
	return RedactionPlaceholder
}

// RedactEnv renders NAME=value as NAME=<value redacted> and drops the value.
func RedactEnv(pair string) string {
	if i := indexByte(pair, '='); i >= 0 {
		return pair[:i] + "=" + RedactionPlaceholder
	}
	return pair
}

// indexByte avoids importing strings for one call; the compiler inlines this.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
