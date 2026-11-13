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

# 2. One name. The project once carried three spellings at the same time
#    (repository, Marketplace listing, prose), and the install line and the
#    `uses:` example disagreed with each other. The module path is the anchor:
#    the Action's `name:`, the `cmd/` directory and the install line all derive
#    from it, so a rename that misses one of them fails here instead of on a
#    user's first `go install`.
module=$(awk '/^module /{print $2; exit}' go.mod)
if [ -z "$module" ]; then
  err "go.mod has no module line"
else
  base=${module##*/}
  action_name=$(awk '/^name: /{print $2; exit}' action.yml)
  if [ "$action_name" != "$base" ]; then
    err "action.yml is named '$action_name' but the module is '$module'"
  fi
  [ -d "cmd/$base" ] || err "cmd/$base does not exist (module basename is '$base')"
  if ! grep -q "go install $module/cmd/$base@" "$README"; then
    err "$README has no 'go install $module/cmd/$base@<tag>' line"
  fi
  repo=${module#github.com/}
  if ! grep -qE "uses: *$repo@" "$README"; then
    err "$README has no 'uses: $repo@<ref>' example"
  fi
fi

# 3. The action example must pin a tag that actually resolves.
# README only: it is the canonical example, and prose elsewhere may quote the
# ref shape (`emrelab/go-flake-hunter@<ref>`) without claiming anything is pinned.
ref=$(grep -hoE 'uses: *emrelab/go-flake-hunter@[^ #]+' "$README" \
      | head -1 | sed 's/.*@//')
if [ -z "$ref" ]; then
  err "no 'uses: emrelab/go-flake-hunter@<ref>' example found in $README"
elif [ "$ref" = "main" ] || [ "$ref" = "master" ]; then
  err "the action example pins the mutable branch '$ref'"
elif ! git rev-parse -q --verify "refs/tags/$ref" >/dev/null; then
  err "the action example pins '$ref', which is not a tag in this clone"
fi

# 4. The sample hunt output must be the output the binary produces.
[ -x ./go-flake-hunter ] || { echo "docs-drift: build first (make build)" >&2; exit 2; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

( cd testdata/fixtures/order-basic && "$root/go-flake-hunter" hunt ./... ) \
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
      -e 's/gfh_[0-9a-f]{12}/gfh_ID/g' \
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
