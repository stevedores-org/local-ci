# local-ci

A lightweight, cacheable local CI runner for Rust workspaces. Mirrors GitHub Actions with file-hash caching and supports optional cargo ecosystem tools.

## Features

- 🚀 **Fast**: File-hash caching skips unchanged checks
- 🎨 **Colored Output**: Visual feedback with GitHub Actions-style formatting
- 📦 **Minimal**: Single binary with zero dependencies (except TOML parsing)
- 🔧 **Flexible**: Run specific stages or all stages
- 🛠️ **Tool Support**: Integrates with cargo ecosystem (deny, audit, machete, taplo)
- 📂 **Workspace Aware**: Auto-detects workspace structure and excludes
- ⚡ **Config-Driven**: `.local-ci.toml` for per-project customization
- 🪝 **Git Hooks**: Optional pre-commit hook generation

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

Initialize local-ci in your Rust project:

```bash
cd your-rust-project
local-ci init
```

This will:
1. Create `.local-ci.toml` with sensible defaults
2. Add `.local-ci-cache` to `.gitignore`
3. Generate optional pre-commit hook

Then run:

```bash
local-ci
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

| Stage | Command | Check | Auto-fix |
|-------|---------|-------|----------|
| **fmt** | `cargo fmt --check` | ✓ | ✓ |
| **clippy** | `cargo clippy -D warnings` | ✗ | ✗ |
| **test** | `cargo test --workspace` | ✗ | ✗ |
| **check** | `cargo check --workspace` | ✗ | ✗ |

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

### Default Behavior

If `.local-ci.toml` doesn't exist, local-ci uses sensible defaults:
- Runs: fmt, clippy, test
- Skips: .git, target, .github, scripts, .claude
- Hashes: *.rs, *.toml files
- Timeout: 30s default (per-stage configurable)

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

## Workspace Support

local-ci automatically detects workspace structure from `Cargo.toml`:

```toml
[workspace]
members = ["crates/*"]
exclude = ["crates/legacy-*"]
```

**Auto-detected:**
- Workspace members
- Excluded crates (skipped in hash computation)
- Single-crate projects

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

## License

MIT
