# gamekit

Go packages for finding PC games on Windows, shared by Seaglass and Syncer: Valve VDF parsing, Steam discovery and the Ludusavi manifest.

## Stack
- Language/runtime: Go 1.26 (`go.mod`), single module `github.com/ApolloF/gamekit`; one dependency, `golang.org/x/sys`.
- Framework: none, a library.
- Data: none. Parsers read VDF, ACF and the Ludusavi YAML manifest; golden files live in `*/testdata`.
- Where it runs: imported by Seaglass and Syncer (Windows desktop). Linux builds only need to compile (`GOOS=linux go vet`).

## Commands
- Install: `go mod download`
- Dev/run: none, this is a library
- Build: `go build ./...`
- Lint/typecheck: `go vet ./... && GOOS=linux go vet ./...`
- Test: `go test ./...` (`go test -race ./...` needs cgo)
- Rewrite golden files after an intended change: `go test ./... -update`
- Fuzz: `go test ./vdf -run '^$' -fuzz '^FuzzParse$' -fuzztime 5m` (also `FuzzQuoted`, `FuzzBare`, `FuzzParseBinary`; `./ludusavi` has `FuzzParse`)

## Check command
`go vet ./... && GOOS=linux go vet ./... && go test ./...`
The `checker` agent and CI (`.github/workflows/test.yml`) run these steps. Keep it fast and keep it passing.

## Layout
- `vdf`: Valve KeyValues, text and binary (`shortcuts.vdf`), read and written without disturbing what Steam put there.
- `steam`: Steam directory, library folders, installed apps, accounts, running state, tamper detection. Windows-only code sits in `*_windows.go`, with `*_other.go` fallbacks.
- `ludusavi`: parser for the Ludusavi manifest (ids, install dirs, save locations, cloud support).
- `*/testdata`: golden files and inputs. `*/testdata/fuzz` holds inputs a fuzz target once failed on; every `go test` runs them.

## Conventions
- Every parser treats its input as untrusted: sizes capped, nesting bounded, fuzz-tested. Keep it that way for new parsers.
- The packages only read; they never download or change anything on their own. Writing files back is the caller's job.
- Table tests (`table_test.go`) and runnable examples (`example_test.go`) are the test style. Golden files are compared byte for byte (`testdata` is `-text` in `.gitattributes`).
- Public API changes affect Seaglass and Syncer; keep them backward compatible or say so in the PR.
- Windows-only code needs a non-Windows counterpart so `GOOS=linux go vet ./...` stays green.

## Definition of done
- The check command passes, with no new warnings.
- Tests were added or updated for changed behaviour; golden files were regenerated only for intended changes.
- New parsers have a fuzz target and its corpus in `testdata/fuzz`.

## Git workflow
- Remote: GitHub ApolloF/gamekit.
- Branch, PR, CI (`test` job), then merge only when the user says "ship it" (auto-merge, squash).
- Releases: tag `v*` on main; consumers pin the module version.

## Secrets
- No config or secrets. If one is ever needed it comes from env vars; `.env.example` would list the keys with no values.
- Never commit real values. gitleaks runs on every commit.
