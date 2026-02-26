# Contributing to local-ci

## Quick Start

```bash
git clone https://github.com/stevedores-org/local-ci
cd local-ci
go test -v ./...    # verify tests pass
go build -o local-ci .
```

## Development Workflow

We follow **TDD (Test-Driven Development)**:

1. Write a failing test in `local_ci_test.go` or `nix_cache_test.go`
2. Run `go test -v ./...` — confirm it fails
3. Implement the feature
4. Run `go test -v ./...` — confirm it passes
5. Open a PR

## Branch Naming

```
claude-code/feat-<description>    # feature branches
claude-code/fix-<description>     # bug fixes
```

## PR Labels

Tag PRs with: `claude`, `claude-code`, `tdd`, and the model used (e.g., `opus-4.6`).

## Project Structure

This is a flat Go project — everything lives in `package main`:

```
main.go             CLI entry point, stage execution engine
config.go           Configuration (.local-ci.toml parsing)
workspace.go        Cargo workspace detection
toolcheck.go        Optional tool detection
hooks.go            Git hook management
nix-cache.go        Nix binary cache support
local_ci_test.go    Core test suite
nix_cache_test.go   Nix cache test suite
```

## Testing

```bash
# Run all tests
go test -v ./...

# Run benchmarks
go test -bench=. ./...

# Run a specific test
go test -v -run TestLoadConfigDefaults ./...
```

Tests that require Nix are automatically skipped when Nix is not installed.

## Code Style

- Keep it simple — this is a single-binary CLI tool
- No sub-packages; everything is `package main`
- One external dependency: `BurntSushi/toml`
- Test helpers accept `testing.TB` (not `*testing.T`) for benchmark compatibility
- Never append directly to package-level slices — copy first

## Deploying to a Rust Repo

```bash
cd your-rust-workspace
local-ci init       # creates .local-ci.toml, updates .gitignore, adds pre-commit hook
local-ci            # run default stages (fmt, clippy, test)
```

## Supported Repos

local-ci is designed for all stevedores-org Rust workspaces:

- [llama.rs](https://github.com/stevedores-org/llama.rs) — inference runtime
- [oxidizedRAG](https://github.com/stevedores-org/oxidizedRAG) — GraphRAG
- [oxidizedMLX](https://github.com/stevedores-org/oxidizedMLX) — MLX bindings
- [oxidizedgraph](https://github.com/stevedores-org/oxidizedgraph) — LangGraph
- [aivcs](https://github.com/stevedores-org/aivcs) — AI version control
- [DevProd-AI](https://github.com/stevedores-org/DevProd-AI) — DevProd tooling
