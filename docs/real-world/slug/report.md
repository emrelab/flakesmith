### flake: TestSlugMake

| field | value |
|---|---|
| package | github.com/gosimple/slug |
| class | order |
| polluter | TestSlugMakeUserSubstituteLang |
| run | 3/3 failed |
| status | confirmed |
| capsule | `fs_8fbbcdffd315` |

Reproduce:

```sh
go test . -run '^TestFlakeWitness$' -count=1 -parallel=1
```

Verification: `verified`.

Permanent regression test: `slug_TestSlugMake_flakesmith_test.go` (build tag `flakesmith`).
