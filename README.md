# local-ci

A lightweight, cacheable local CI runner for Rust workspaces. Mirrors GitHub Actions (fmt, clippy, test) with file-hash caching for fast iteration.

## Features

- 🚀 **Fast**: File-hash caching skips unchanged checks
- 🎨 **Colored Output**: Visual feedback for pass/fail/warning
- 📦 **Minimal**: Single binary, no dependencies
- 🔧 **Flexible**: Run specific stages or all stages

## Installation

```bash
go install github.com/stevedores-org/local-ci@latest
```

Or clone and build:

```bash
git clone https://github.com/stevedores-org/local-ci
cd local-ci
go build -o local-ci
```

## Nix / Attic Cache

This repo includes a `flake.nix` configured with:
- `nixConfig.extra-substituters = [ "https://nix-cache.stevedores.org" ]`

Use:

```bash
nix develop
go build -o local-ci
```

## Usage

```bash
# Run default stages (fmt, clippy, test)
local-ci

# Run specific stages
local-ci fmt clippy

# Emit machine-readable output for agents
local-ci --json

# Disable cache
local-ci --no-cache

# Auto-fix formatting
local-ci --fix

# Stop at first failure
local-ci --fail-fast

# Verbose output
local-ci --verbose

# List available stages
local-ci --list

# Print version
local-ci --version
```

## Available Stages

- **fmt**: Format check (cargo fmt --check)
- **clippy**: Linter (cargo clippy -D warnings)
- **test**: Tests (cargo test --workspace)
- **check**: Compile check (cargo check)

Unknown stage names now fail fast (instead of being silently ignored).

## Caching

Cache is stored in `.local-ci-cache` and based on MD5 hash of all `.rs` and `.toml` files. Cache is skipped for:
- `.git`, `target`, `.github`, `scripts`, `.claude` directories

Each stage cache entry includes the command signature, so changing command args invalidates stale cache entries automatically.

## Agent/Automation Mode

Use `--json` for deterministic machine-readable output:

```bash
local-ci --json
```

Sample schema:
- `version`
- `duration_ms`
- `passed`
- `failed`
- `results[]` with `name`, `command`, `status`, `duration_ms`, `cache_hit`, optional `output`, optional `error`

## License

MIT
