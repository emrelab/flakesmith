---
render_with_liquid: false
layout: default
title: Real-world witness validation
---

# Real-world witness validation

Validated on November 10, 2026, macOS ARM64, Go 1.27.3, against two real
repositories with upstream test-isolation reports. Both checkouts were clean
when searched. No test body was modified to make discovery succeed.

| Case | Base commit | Upstream fix or proposal | Replay before | Compile before | Generated test after fix | Replay after |
|---|---|---|---|---|---|---|
| [slug](slug/summary.json) | `1eab3167af84fc96dbabe7b5d9c22d6d4c214800` | [gosimple/slug#83](https://github.com/gosimple/slug/pull/83), `76470d7e1c9c8bb5cf9b9df392b9fb5f35b77e9a` | 5/5 failed | verified RED | PASS | 0/2, exit 3 |
| [cliproxy](cliproxy/summary.json) | `0074897481fee1dcd595c1caa1d57e6a379b6be6` | [CLIProxyAPI#7356](https://github.com/router-for-me/CLIProxyAPI/pull/7356), `e056459bd00de0c63bbf5e09879704b0d53fd142` | 5/5 failed | verified RED | PASS | 0/2, exit 3 |

These targeted historical cases demonstrate reproduction and permanence. They
do not estimate recall or false-positive rate over arbitrary repositories.
Only the targeted generated regression is claimed green after each fix; no
claim is made that the entire upstream suite is green. `gosimple/slug#83` is
merged (2024); `CLIProxyAPI#7356` is an open proposal, applied locally from its
patch excerpt to prove the regression turns green.

## Repeat the validation

From the flakesmith checkout:

```sh
scripts/verify-real-world.sh /tmp/flakesmith-real-world-evidence
```

The script builds flakesmith from the current source, creates disposable
checkouts, downloads each repository's pinned dependencies, records discovery,
replays five times, generates and verifies the regression, applies the exact
upstream test-file patch, and runs the same generated regression again. It also
requires replay to return exit 3 after the fix. Network access and support for
local HTTP test listeners are required. Output files are retained; disposable
checkouts are removed.

Each case directory contains the confirmed capsule with `verifyStatus`, the
search report, replay outputs, compile verification output, generated regression
source (as text), the upstream patch and the green result. Upstream patches are
small excerpts from the linked projects; their source remains under the
[slug MPL 2.0 license](https://github.com/gosimple/slug/blob/master/LICENSE) and
[CLIProxyAPI MIT license](https://github.com/router-for-me/CLIProxyAPI/blob/main/LICENSE).

## slug: custom substitutions leak into a default slug test

Package: repository root. Minimal witness:

```text
TestSlugMakeUserSubstituteLang → TestSlugMake
```

The custom substitution test leaves its `CustomSub` map active. A later default
slug conversion observes the custom replacement. Upstream #83 resets the
custom substitution maps; applying that patch makes this generated regression
pass.

Discovery command, from the pinned upstream checkout:

```sh
GOFLAGS=-shuffle=3 flakesmith repro . TestSlugMake --budget 200 --timeout 5m \
  --capsule-dir /tmp/slug-capsules --format json
```

The seed exposes the reported custom-substitution relationship before the
package's separate truncation-state leak can obscure it. The seed is only a
search aid: replay and the generated regression run without it, using the
explicit polluter→victim chain.

## CLIProxyAPI: a websocket client leaks into the model catalog

Package: `sdk/api/handlers/openai`. Minimal witness:

```text
TestResponsesWebsocketHomeSelectedAuthCallbackPinsAndReusesFirstSelection
  → TestCodexClientModelsApplyPatchRouting
```

The websocket test registers a process-global model-registry client and leaves
it present. The catalog test later sees an unexpected model/capability entry.
Upstream #7356 adds `t.Cleanup` to unregister that client. The same generated
regression passes with that patch applied.

Discovery command, from the pinned package directory:

```sh
GOFLAGS='-run=^(TestCodexClientModelsApplyPatchRouting|TestResponsesWebsocketHomeSelectedAuthCallbackPinsAndReusesFirstSelection)$' \
  flakesmith repro . TestCodexClientModelsApplyPatchRouting --budget 200 --timeout 5m \
  --capsule-dir /tmp/cliproxy-capsules --format json
```

The package sweep is scoped to the two tests named in the upstream report;
isolated and witness runs supply their own `-run`, overriding that default.
Replay and generated-test verification need no search filter. An unrestricted
package search also found the same pair during investigation, but the repeatable
script uses the narrower scope to avoid unrelated slow/timing-sensitive tests.

## Local integration reductions

`testdata/fixtures/substitution-leak` and `testdata/fixtures/registry-leak` are
original stdlib-only models of these mechanisms. They are exercised by
`TestCompileVerifyReproducesEachClass`. These reductions are not counted as
real-world findings; the pinned upstream checkouts above supply that evidence.
