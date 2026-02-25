# local-ci

A lightweight, cacheable local CI runner for Rust and TypeScript/Bun workspaces. Auto-detects project type, mirrors GitHub Actions with file-hash caching, supports optional cargo ecosystem tools, and provides first-class TypeScript/Bun pipeline stages.

## Features

- 🚀 **Fast**: File-hash caching skips unchanged checks
- 🎨 **Colored Output**: Visual feedback with GitHub Actions-style formatting
- 📦 **Minimal**: Single binary with zero dependencies (except TOML parsing)
- 🔧 **Flexible**: Run specific stages or all stages
- 🛠️ **Tool Support**: Integrates with cargo ecosystem (deny, audit, machete, taplo) and Bun/TypeScript tooling
- 📂 **Workspace Aware**: Auto-detects Rust workspace structure and Bun/TypeScript projects
- ⚡ **Config-Driven**: `.local-ci.toml` for per-project customization
- 🪝 **Git Hooks**: Optional pre-commit hook generation
- 🔗 **Nix Cache Integration**: Optional attic cache support for faster builds (nix-cache.stevedores.org)

## Installation

### From Source

```bash
git clone https://github.com/stevedores-org/local-ci
cd local-ci
go build -o local-ci
# Copy binary to PATH
sudo cp local-ci /usr/local/bin/
```

### Using Go

```bash
go install github.com/stevedores-org/local-ci@latest
```

## Quick Start

Initialize local-ci in your project (auto-detects Rust or TypeScript/Bun):

```bash
cd your-project
local-ci init
```

This will:
1. Detect project type (Cargo.toml → Rust, package.json + tsconfig/bunfig/bun.lock → TypeScript)
2. Create `.local-ci.toml` with language-appropriate defaults
3. Add `.local-ci-cache` to `.gitignore`
4. Generate optional pre-commit hook

Then run:

```bash
local-ci
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

### Basic Commands

```bash
# Run default stages (fmt, clippy, test)
local-ci

# Run specific stages
local-ci fmt clippy

# Initialize project
local-ci init

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

### Flags

```bash
--no-cache      Disable caching, run all stages
--fix           Auto-fix issues (e.g., cargo fmt without --check)
--verbose       Show detailed output including command execution
--all           Run all stages including disabled ones
```

## Default Stages

### Rust (auto-detected via Cargo.toml)

| Stage | Command | Check | Auto-fix |
|-------|---------|-------|----------|
| **fmt** | `cargo fmt --check` | ✓ | ✓ |
| **clippy** | `cargo clippy -D warnings` | ✗ | ✗ |
| **test** | `cargo test --workspace` | ✗ | ✗ |
| **check** | `cargo check --workspace` | ✗ | ✗ |

### TypeScript/Bun (auto-detected via package.json + tsconfig/bunfig/bun.lock)

| Stage | Command | Auto-fix | Default |
|-------|---------|----------|---------|
| **typecheck** | `bun x tsc --noEmit` | ✗ | enabled |
| **lint** | `bun run lint` | ✓ (`--fix`) | enabled |
| **test** | `bun test` | ✗ | enabled |
| **format** | `bun run format --check` | ✓ | disabled |

Lint and format stages delegate to your `package.json` scripts, so you can use eslint, biome, prettier, or any other tool.

## Optional Cargo Tool Stages

### cargo-deny (Security & License Checking)

Check dependencies for security vulnerabilities and license compliance:

```bash
cargo install cargo-deny

# Enable in .local-ci.toml
# [stages.deny]
# enabled = true

# Run standalone
local-ci deny
```

### cargo-audit (CVE Scanning)

Audit dependencies for known CVEs:

```bash
cargo install cargo-audit

# Enable in .local-ci.toml
# [stages.audit]
# enabled = true

local-ci audit
```

### cargo-machete (Unused Dependencies)

Find unused dependencies:

```bash
cargo install cargo-machete

# Enable in .local-ci.toml
# [stages.machete]
# enabled = true

local-ci machete
```

### taplo (TOML Formatting)

Format and lint TOML files:

```bash
cargo install taplo-cli

# Enable in .local-ci.toml
# [stages.taplo]
# enabled = true

# Format with fix
local-ci --fix taplo

# Check formatting
local-ci taplo
```

## Configuration

### .local-ci.toml

Each project can have a `.local-ci.toml` configuration file:

```toml
[cache]
# Directories to skip when computing source hash
skip_dirs = [".git", "target", ".github", "scripts", ".claude"]
# File patterns to include in hash
include_patterns = ["*.rs", "*.toml"]

[stages.fmt]
command = ["cargo", "fmt", "--all", "--", "--check"]
fix_command = ["cargo", "fmt", "--all"]
timeout = 120
enabled = true

[stages.clippy]
command = ["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"]
timeout = 600
enabled = true

[stages.test]
command = ["cargo", "test", "--workspace"]
timeout = 1200
enabled = true

# Optional tool stages...
[stages.deny]
command = ["cargo", "deny", "check"]
timeout = 300
enabled = false

[dependencies]
# System dependencies (optional)
required = []
optional = []

[workspace]
# Workspace members to exclude
exclude = []
```

### TypeScript/Bun .local-ci.toml

```toml
[cache]
skip_dirs = [".git", "node_modules", "dist", ".next", "coverage", ".claude"]
include_patterns = ["*.ts", "*.tsx", "*.js", "*.jsx", "*.json"]

[stages.typecheck]
command = ["bun", "x", "tsc", "--noEmit"]
timeout = 120
enabled = true

[stages.lint]
command = ["bun", "run", "lint"]
fix_command = ["bun", "run", "lint", "--", "--fix"]
timeout = 300
enabled = true

[stages.test]
command = ["bun", "test"]
timeout = 600
enabled = true

[stages.format]
command = ["bun", "run", "format", "--check"]
fix_command = ["bun", "run", "format"]
timeout = 120
enabled = false
```

### Default Behavior

If `.local-ci.toml` doesn't exist, local-ci auto-detects the project type and uses sensible defaults:

**Rust projects:**
- Runs: fmt, clippy, test
- Skips: .git, target, .github, scripts, .claude
- Hashes: *.rs, *.toml files

**TypeScript/Bun projects:**
- Runs: typecheck, lint, test
- Skips: .git, node_modules, dist, .next, coverage, .claude
- Hashes: *.ts, *.tsx, *.js, *.jsx, *.json files

Unknown stage names now fail fast (instead of being silently ignored).

## Caching

Cache is stored in `.local-ci-cache` (added to `.gitignore` by `local-ci init`).

**How it works:**
1. Compute MD5 hash of all Rust files in workspace
2. Skip stages if source hash matches cached hash
3. Update cache when stage succeeds

**Skip directories:**
- `.git`, `target`, `.github`, `scripts`, `.claude` (configurable)

**Force rebuild:**
```bash
local-ci --no-cache
```

## Pre-commit Hook

Initialize with optional Git pre-commit hook:

```bash
local-ci init
# Creates .git/hooks/pre-commit
```

Hook runs `local-ci fmt clippy` before allowing commits. Modify hook in `.git/hooks/pre-commit`.

Bypass hook:
```bash
git commit --no-verify
```

## Nix Cache Configuration

local-ci supports attic binary cache integration for faster Nix builds:

```bash
# Configure stevedores attic cache
local-ci configure-nix-cache
```

This adds `https://nix-cache.stevedores.org` to your Nix configuration for faster builds across all stevedores-org projects.

**Supported Caches:**
- `stevedores-attic` (https://nix-cache.stevedores.org) - Trusted cache for stevedores-org ecosystem
- `cache.nixos.org` - Official NixOS binary cache

**Manual Configuration:**

Add to `~/.config/nix/nix.conf`:
```ini
extra-substituters = https://nix-cache.stevedores.org https://cache.nixos.org
trusted-public-keys = oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI= cache.nixos.org-1:6NCHdD59X431o0gWypQydGvjwydGG2UZTvhjGJNsx6E=
```

Or system-wide in `/etc/nix/nix.conf` (requires root)

## Workspace Support

local-ci automatically detects workspace structure from `Cargo.toml` (Rust) or `package.json` workspaces (TypeScript/Bun).

### Rust

```toml
[workspace]
members = ["crates/*"]
exclude = ["crates/legacy-*"]
```

**Auto-detected:**
- Workspace members
- Excluded crates (skipped in hash computation)
- Single-crate projects

### TypeScript/Bun

```json
{
  "workspaces": ["packages/*", "apps/*"]
}
```

**Auto-detected:**
- Workspace members from package.json `workspaces` globs
- Single-package projects (no workspaces field)

Configure in `.local-ci.toml`:
```toml
[workspace]
exclude = ["crates/experimental"]
```

## Output Format

```
🚀 Running local CI pipeline...

::group::fmt
$ cargo fmt --all -- --check
::endgroup::
✓ fmt (125ms)

::group::clippy
$ cargo clippy --workspace --all-targets -- -D warnings
::endgroup::
✓ clippy (2345ms)

::group::test
$ cargo test --workspace
::endgroup::
✓ test (5678ms)

📊 Summary:
  Total stages: 3
  Passed: 3
  Cached: 1 (33%)
  Executed: 2
  Total time: 8123ms

💡 Optional tools missing:
  cargo-deny:
    cargo install cargo-deny
```

## Examples

### Basic Usage

```bash
# Test before committing
local-ci

# Run single stage
local-ci fmt

# Auto-fix formatting
local-ci --fix
```

### Continuous Integration

Run in GitHub Actions:

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: dtolnay/rust-toolchain@stable
      - run: cargo install --locked local-ci
      - run: local-ci
```

### Development Workflow

1. Make changes to code
2. Run `local-ci` to check before pushing
3. Fix any issues (use `local-ci --fix` for formatting)
4. Commit with pre-commit hook running checks

## Troubleshooting

### "local-ci not found"

Ensure binary is in PATH:
```bash
which local-ci
export PATH="$PATH:$(go env GOPATH)/bin"  # Add to .bashrc or .zshrc
```

### Stage fails but works manually

Check if `.local-ci.toml` has correct command:
```bash
local-ci --verbose stagename
```

### Cache causing issues

Clear cache:
```bash
rm .local-ci-cache
local-ci --no-cache
```

### Missing cargo tools

Install missing tools shown in output:
```bash
cargo install cargo-deny cargo-audit cargo-machete
```

## Contributing

Bug reports and PRs welcome on GitHub: https://github.com/stevedores-org/local-ci

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
