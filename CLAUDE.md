# local-ci — Claude Code Instructions

## Project Overview

Go CLI tool that provides a fast, cacheable local CI pipeline for Rust and TypeScript/Bun workspaces. Mirrors GitHub Actions stages with file-hash caching, config-driven stages, and workspace awareness. Auto-detects project type from `Cargo.toml` (Rust) or `package.json` + TS/Bun indicators (TypeScript).

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
| `config.go` | `.local-ci.toml` parsing, default Rust stages, `Config` struct |
| `workspace.go` | `Cargo.toml` workspace detection, glob expansion |
| `project.go` | `ProjectKind` enum, `DetectProjectKind` (Rust vs TypeScript vs Unknown) |
| `typescript.go` | TS/Bun workspace detection, default TS stages, TS config templates |
| `toolcheck.go` | Tool detection for cargo and bun ecosystems (deny, audit, machete, taplo) |
| `hooks.go` | Git pre-commit hook creation/removal |
| `nix-cache.go` | Nix binary cache (attic) configuration |

## Key Types

- `ProjectKind` — `rust`, `typescript`, or `unknown`
- `Stage` — CI stage with `Cmd`, `FixCmd`, `Timeout`, `Enabled`
- `Config` — parsed from `.local-ci.toml` (cache, stages, workspace, deps)
- `Workspace` — workspace structure (members, excludes, single detection) — shared by Rust and TS
- `PackageJSON` — parsed fields from `package.json` (name, workspaces, scripts)
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

This tool is designed for all stevedores-org repos:
- **Rust**: `llama.rs`, `oxidizedRAG`, `oxidizedMLX`, `oxidizedgraph`, `aivcs`, `DevProd-AI`
- **TypeScript/Bun**: `cousin-cli`, and any project with `package.json` + `tsconfig.json`/`bunfig.toml`/`bun.lock`

Nix cache: `https://nix-cache.stevedores.org` (attic)

## Common Tasks

```bash
# Add a new Rust stage: edit defaultStages() in config.go
# Add a new TS/Bun stage: edit defaultTypeScriptStages() in typescript.go
# Add a new tool check: append to cargoTools/bunTools/systemTools in toolcheck.go
# Modify hashing: edit computeSourceHash() in main.go
# Modify workspace detection: edit DetectWorkspace() or DetectTypeScriptWorkspace()
# Add new project type: extend ProjectKind in project.go
```
