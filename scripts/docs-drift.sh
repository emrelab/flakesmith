#!/usr/bin/env bash
# Fails when README/docs claims diverge from the binary and the tag list.
# Why this exists: the README said "no release has been cut yet" for a day after
# v0.2.0 shipped, and its sample hunt output lost the Wilson line. Both were
# invisible to `go test` and to every reviewer who did not run the tool.
set -uo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
README=${README:-README.md}
fail=0
err() { echo "docs-drift: $*" >&2; fail=1; }

# 1. Claims that a release does not exist.
if grep -rn "cut yet\|has been cut\|not been published" --include='*.md' . \
   | grep -v CHANGELOG.md | grep -v 'docs-drift'; then
  err "a doc still claims no release exists"
fi

# 2. The action example must pin a tag that actually resolves.
ref=$(grep -rhoE 'uses: *emrelab/flakesmith@[^ #]+' . --include='*.md' \
      | head -1 | sed 's/.*@//')
if [ -z "$ref" ]; then
  err "no 'uses: emrelab/flakesmith@<ref>' example found in the docs"
elif [ "$ref" = "main" ] || [ "$ref" = "master" ]; then
  err "the action example pins the mutable branch '$ref'"
elif ! git rev-parse -q --verify "refs/tags/$ref" >/dev/null; then
  err "the action example pins '$ref', which is not a tag in this clone"
fi

# 3. The sample hunt output must be the output the binary produces.
[ -x ./flakesmith ] || { echo "docs-drift: build first (make build)" >&2; exit 2; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

( cd testdata/fixtures/order-basic && "$root/flakesmith" hunt ./... ) \
  > "$tmp/actual.raw" 2>&1
# Only the confirmed block; the run also reports the tests it cleared.
sed -n '/^flake found: TestVictim/,/status .*: confirmed/p' "$tmp/actual.raw" \
  > "$tmp/actual"

# The documented block: between the first fence pair after "Real output".
awk '
  /^Real output/ { seen = 1; next }
  seen && /^```/ { if (inb) exit; inb = 1; next }
  inb { print }
' "$README" > "$tmp/doc"

if [ ! -s "$tmp/actual" ] || [ ! -s "$tmp/doc" ]; then
  err "could not extract the sample output (doc block or fixture run empty)"
else
  norm() {
    sed -E \
      -e 's/fs_[0-9a-f]{12}/fs_ID/g' \
      -e 's/^[[:space:]]+//' -e 's/[[:space:]]+: /: /' \
      -e 's/^run: .*/run: N\/N/' \
      -e 's/^Wilson 95%: .*/Wilson 95%: X/' \
      -e '/^$/d' "$1"
  }
  if ! diff -u <(norm "$tmp/doc") <(norm "$tmp/actual") > "$tmp/diff"; then
    err "the README sample output differs from the binary's"
    sed -n '1,40p' "$tmp/diff" >&2
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "docs-drift: FAILED — update the docs or the fixture, not this script" >&2
  exit 1
fi
echo "docs-drift: ok"
