### flake: TestCodexClientModelsApplyPatchRouting

| field | value |
|---|---|
| package | github.com/router-for-me/CLIProxyAPI/v8/sdk/api/handlers/openai |
| class | order |
| polluter | TestResponsesWebsocketHomeSelectedAuthCallbackPinsAndReusesFirstSelection |
| run | 3/3 failed |
| status | confirmed |
| capsule | `fs_45d3dfda4ce1` |

Reproduce:

```sh
go test . -run '^TestFlakeWitness$' -count=1 -parallel=1
```

Verification: `verified`.

Permanent regression test: `codex_client_models_TestCodexClientModelsApplyPatchRouting_flakesmith_test.go` (build tag `flakesmith`).
