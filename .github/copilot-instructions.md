# GitHub Copilot Instructions — local-ci

## Context

Go CLI tool for running local CI on Rust workspaces. Single `package main`, one external dep (`BurntSushi/toml`).

## Code Generation Rules

- Generate Go code only
- Keep everything in `package main` — no sub-packages
- Test helpers must accept `testing.TB`, not `*testing.T`
- Never append to package-level slices directly (copy first)
- Use `t.Skip()` for tests that depend on Nix being installed
- Follow TDD: write test first, then implementation

## Key Commands

```bash
go test -v ./...     # run tests
go build -o local-ci # build binary
make test            # run tests via Makefile
```

## File Responsibilities

- `main.go` — CLI, stage execution, hashing
- `config.go` — TOML config parsing
- `workspace.go` — Cargo workspace detection
- `toolcheck.go` — tool availability checks
- `hooks.go` — git pre-commit hooks
- `nix-cache.go` — Nix cache setup
