#!/usr/bin/env bash
# Reproduce two upstream test-isolation bugs without changing their test bodies.
# Network access and local httptest listener support are required. Not a CI unit test.
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:?usage: scripts/verify-real-world.sh OUTPUT_DIRECTORY}
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/flakesmith-proof.XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT
BIN="$SCRATCH/flakesmith"
(cd "$ROOT" && go build -o "$BIN" ./cmd/flakesmith)

verify_case() {
 local name=$1 repo=$2 base=$3 fix=$4 pkg=$5 victim=$6 flags=$7 source=$8 pr=$9 patch_source=${10}
 local checkout="$SCRATCH/$name" artifacts="$OUT/$name"
 mkdir -p "$artifacts"
 git clone --quiet --depth 1 --no-checkout "$repo" "$checkout"
 git -C "$checkout" fetch --quiet --depth 2 origin "$base" "$fix"
 git -C "$checkout" checkout --quiet --detach "$base"
 cp "$checkout/LICENSE" "$artifacts/LICENSE"
 # Persist the exact upstream fix, not a hand-written approximation.
 git -C "$checkout" show "$fix" --format= -- "$pkg/$patch_source" > "$artifacts/upstream-fix.patch"
 [ -s "$artifacts/upstream-fix.patch" ]
 (
  cd "$checkout/$pkg"
  go mod download
  GOFLAGS="$flags" "$BIN" repro . "$victim" --budget 200 --timeout 5m \
   --capsule-dir "$artifacts/capsules" --format json > "$artifacts/repro.json"
  local id
  id=$(python3 - "$artifacts/repro.json" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
assert len(r['flakes']) == 1 and r['flakes'][0]['status'] == 'confirmed', r
assert r['flakes'][0]['repo']['dirty'] is False, 'baseline source must be clean'
print(r['flakes'][0]['id'])
PY
)
  "$BIN" replay "$id" --pkg . --capsule-dir "$artifacts/capsules" -n 5 > "$artifacts/replay.txt"
  "$BIN" compile "$id" --pkg . --capsule-dir "$artifacts/capsules" --verify > "$artifacts/compile.txt"
  "$BIN" explain "$id" --capsule-dir "$artifacts/capsules" --format markdown > "$artifacts/report.md"
  cp "${source%_test.go}_${victim}_flakesmith_test.go" "$artifacts/regression_test.go.txt"
  git -C "$checkout" apply "$artifacts/upstream-fix.patch"
  go test -tags flakesmith -run "^TestFlakeRegression_${victim}$" -count=1 . > "$artifacts/after-fix.txt"
  # A fixed witness must be reported as stale, never as still confirmed.
  local status=0
  "$BIN" replay "$id" --pkg . --capsule-dir "$artifacts/capsules" -n 2 > "$artifacts/after-fix-replay.txt" 2>&1 || status=$?
  [ "$status" -eq 3 ]
 )
 python3 - "$artifacts" "$repo" "$base" "$fix" "$pkg" "$victim" "$flags" "$pr" <<'PY'
import json, pathlib, platform, subprocess, sys
out, repo, base, fix, pkg, victim, flags, pr = sys.argv[1:]
p = pathlib.Path(out)
capsule = json.loads(next((p/'capsules').glob('fs_*.json')).read_text())
assert capsule['verifyStatus'] == 'verified'
summary = dict(repository=repo, upstream=pr, base=base, fix=fix, package=pkg,
               victim=victim, polluters=[x['name'] for x in capsule['witness']['polluters']],
               searchGoFlags=flags, replayBefore='5/5', compileBefore='verified (RED)',
               afterFix='PASS', replayAfter='0/2, exit 3',
               go=subprocess.check_output(['go','version'], text=True).strip(),
               platform=platform.platform())
(p/'summary.json').write_text(json.dumps(summary, indent=2)+'\n')
PY
 echo "$name: confirmed; replay 5/5; regression RED -> GREEN with upstream fix"
}

verify_case slug https://github.com/gosimple/slug.git \
 1eab3167af84fc96dbabe7b5d9c22d6d4c214800 76470d7e1c9c8bb5cf9b9df392b9fb5f35b77e9a \
 . TestSlugMake '-shuffle=3' slug_test.go https://github.com/gosimple/slug/pull/83 slug_test.go

# Scope the package sweep to the two tests named in the upstream report.
# Runner -run flags override this on isolated and generated-witness runs.
verify_case cliproxy https://github.com/router-for-me/CLIProxyAPI.git \
 0074897481fee1dcd595c1caa1d57e6a379b6be6 e056459bd00de0c63bbf5e09879704b0d53fd142 \
 sdk/api/handlers/openai TestCodexClientModelsApplyPatchRouting \
 '-run=^(TestCodexClientModelsApplyPatchRouting|TestResponsesWebsocketHomeSelectedAuthCallbackPinsAndReusesFirstSelection)$' \
 codex_client_models_test.go https://github.com/router-for-me/CLIProxyAPI/pull/7356 openai_responses_websocket_test.go
