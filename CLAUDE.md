# local-ci — Claude Code Instructions

## Project Overview

Go CLI tool that provides a fast, cacheable local CI pipeline for Rust workspaces. Mirrors GitHub Actions stages (fmt, clippy, test) with file-hash caching, config-driven stages, and workspace awareness.

## Build & Test

```bash
# Build
go build -o local-ci .
make build          # versioned build to dist/

# Test
go test -v ./...
make test

# Install
go install .
make install
```

## Architecture

Single-package Go binary (`package main`), no internal packages:

| File | Responsibility |
|------|---------------|
| `main.go` | CLI entry, stage execution, hashing, caching |
| `config.go` | `.local-ci.toml` parsing, default stages, `Config` struct |
| `workspace.go` | `Cargo.toml` workspace detection, glob expansion |
| `toolcheck.go` | Optional cargo tool detection (deny, audit, machete, taplo) |
| `hooks.go` | Git pre-commit hook creation/removal |
| `nix-cache.go` | Nix binary cache (attic) configuration |

## Key Types

- `Stage` — CI stage with `Cmd`, `FixCmd`, `Timeout`, `Enabled`
- `Config` — parsed from `.local-ci.toml` (cache, stages, workspace, deps)
- `Workspace` — Cargo workspace structure (members, excludes, single-crate detection)
- `Result` — stage execution result (status, duration, cache hit)

## Conventions

- **Language**: Go 1.22+, single external dep (`BurntSushi/toml`)
- **Testing**: Standard `go test`, helpers use `testing.TB` interface for test+benchmark compat
- **TDD**: Write tests first — see `local_ci_test.go` and `nix_cache_test.go`
- **Nix-optional tests**: Use `t.Skip()` when Nix is not installed
- **No internal packages**: Everything is `package main` — keep it flat
- **Branch naming**: `claude-code/feat-*`
- **PR labels**: `claude`, `claude-code`, `opus-4.6`, `tdd`

## Ecosystem Integration

This tool is designed for all stevedores-org Rust repos:
- `llama.rs` — 6-crate workspace
- `oxidizedRAG` — 4-crate workspace
- `oxidizedMLX` — 8-crate workspace
- `oxidizedgraph`, `aivcs`, `DevProd-AI`

Nix cache: `https://nix-cache.stevedores.org` (attic)

## Common Tasks

```bash
# Add a new stage: edit defaultStages() in config.go
# Add a new tool check: append to cargoTools or systemTools in toolcheck.go
# Modify hashing: edit computeSourceHash() in main.go
# Modify workspace detection: edit DetectWorkspace() in workspace.go
```
