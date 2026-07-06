# timeBuddy

CLI tool for comparing times across multiple time zones, written in Go.

## Commands

- Build: `go build -o timeBuddy .`
- Test: `go test ./...` (single test: `go test ./cmd -run Test_buildTree`)
- Lint: `golangci-lint run`
- All checks: `pre-commit run --all-files` (lint, race-enabled tests, coverage gate)

## Architecture

- `main.go` — thin wrapper that calls `cmd.Execute()`.
- `cmd/` — Cobra command definitions and CLI entry points: `root.go` (timezone processing, table rendering, live mode), `list.go` (timezone listing), `wizard.go` (interactive Bubbletea timezone picker).
- `logger/` — zerolog wrapper mapping `-v` repetitions to log levels.

## Conventions

- Go style: `.github/instructions/go.instructions.md`. Helpers return wrapped errors; the CLI boundary handles them — commands use `RunE`, never `Run` or `log.Fatal()`.
- Tests are table-driven, live beside the code, and are named `Test_functionName_scenario`.
- Conventional Commits enforced by commitizen (`.cz.yaml`) via the pre-commit `commit-msg` hook; commits and release tags are GPG-signed.

## Gotchas

- The pre-commit coverage hook fails if total test coverage drops below 85%.
- The version string lives in `cmd/root.go`; CI (`verBumpChkr.yml`) fails any PR that does not bump it. It is managed by `cz bump` (`version_files` in `.cz.yaml`).
- `time/tzdata` is embedded via blank import, so tests pass without system timezone data.
- Releases are cut by GoReleaser when a `v*` tag is pushed — never build release artifacts by hand.
