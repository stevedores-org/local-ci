# local-ci — Agent Instructions

Instructions for AI coding agents (Claude, Codex, Copilot, Cursor, Gemini) working on this repo.

## Context

`local-ci` is a Go CLI that runs local CI pipelines for Rust workspaces. It's used across all `stevedores-org` Rust repos to mirror GitHub Actions locally with file-hash caching.

## Before You Code

1. Read `CLAUDE.md` for build/test commands and architecture
2. Run `go test -v ./...` to confirm baseline passes (20 tests pass, 3 skip when Nix absent)
3. Understand the flat structure — everything is `package main`, no internal packages

## Rules

- **TDD**: Write a failing test before implementing features
- **Single package**: Do not create sub-packages. Keep everything in `package main`
- **One external dep**: Only `BurntSushi/toml`. Do not add dependencies without discussion
- **`testing.TB`**: Test helpers must accept `testing.TB`, not `*testing.T`, so benchmarks can reuse them
- **Safe slice ops**: Never `append(packageLevelSlice, ...)` — always copy first to avoid mutating the backing array
- **Deterministic output**: When iterating maps for user-facing output, sort keys first
- **Nix-optional**: Tests that require Nix must use `t.Skip("Nix not installed")` when `CheckNixInstallation()` returns false
- **No secrets**: Never hardcode URLs with auth tokens, API keys, or passwords

## File Map

```
main.go           CLI entry, stage runner, source hashing, cache I/O
config.go         .local-ci.toml schema, LoadConfig(), SaveDefaultConfig()
workspace.go      Cargo.toml parsing, workspace member/exclude detection
project_type.go   ProjectType detection and language-specific defaults
toolcheck.go      cargo-deny/audit/machete/taplo detection
hooks.go          Git pre-commit hook management
nix-cache.go      Nix binary cache (attic) setup
local_ci_test.go  Core tests (14 tests + 2 benchmarks + 1 perf test)
nix_cache_test.go Nix cache tests (9 tests, 3 skip without Nix)
Makefile          build, test, install, clean targets
```

## Adding a New CI Stage

1. Add entry to appropriate `get<Type>Stages()` in `project_type.go`
2. If it requires a tool, add to `cargoTools` or `systemTools` in `toolcheck.go`
3. Add test in `local_ci_test.go`
4. Update `README.md` stage table

## Adding a New Tool Check

1. Append to `cargoTools` or `systemTools` in `toolcheck.go`
2. Ensure `CheckArgs` actually validates the tool is installed (not just that the binary exists)
3. Add install hint string

## Known Issues (from code review)

These are tracked and may need fixing:

- `GetEnabledStages()` returns non-deterministic order (map iteration)
- `StageConfig` type in `config.go` is unused dead code
- `hooks.go` section removal is fragile (needs start/end markers)
- `.local-ci-cache` empty file is committed (should only be in `.gitignore`)
- `config.go` TOML template has `node_modules` in skip_dirs but programmatic defaults don't
