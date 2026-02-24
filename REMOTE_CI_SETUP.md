# Remote CI Setup: local-ci on aivcs@aivcs.local

## Overview

This guide configures **local-ci** to run as a remote CI pipeline on **aivcs@aivcs.local** (Mac Studio) via SSH+tmux, enabling:

- **Distributed CI execution**: Run expensive builds (cargo test, etc.) on powerful hardware
- **Workspace isolation**: Each developer gets isolated tmux sessions
- **Persistent execution**: Builds continue even if SSH connection drops (autossh handles reconnection)
- **Local feedback**: Developers monitor remote builds from their local machine

## Architecture

```
Developer Machine                      aivcs@aivcs.local (Mac Studio)
    │                                        │
    ├─ local-ci CLI                        ├─ Rust environment
    │  (with --remote flag)                 │  (cargo, rustup)
    │                                        │
    ├─ autossh tunnel                       ├─ tmux sessions
    │  (keeps SSH alive)                    │  (one per developer)
    │                                        │
    └─ Monitor output                       ├─ Nix cache (attic)
       (via tmux capture-pane)              │
                                            └─ Build artifacts
```

## Quick Start

### 1. Install local-ci on Remote Machine

```bash
# On aivcs.local
ssh aivcs@aivcs.local
cd /tmp
git clone https://github.com/stevedores-org/local-ci
cd local-ci
go build -o /usr/local/bin/local-ci
```

### 2. Create Autossh Tunnel

```bash
# On your local machine
# Creates persistent SSH tunnel with monitoring on port 0 (disabled)
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
```

**Flags explained:**
- `-M 0`: Disable monitoring port (server doesn't listen)
- `-t`: Allocate pseudo-terminal (required for tmux)
- `new-session -A`: Create session or attach if already exists
- `-s onion`: Session name (change per project)

### 3. Run CI Remotely

In the tmux session:

```bash
cd /path/to/rust-project
local-ci
```

Or with specific stages:

```bash
local-ci fmt clippy test
```

## Remote Execution Workflow

### Without Remote Support (Current)

1. Developer runs `local-ci` locally
2. All stages execute on local machine
3. For large workspaces, this takes 5-10 minutes
4. Local machine is occupied during build

### With Remote Support (Proposed)

1. Developer runs `local-ci --remote aivcs@aivcs.local`
2. local-ci detects `--remote` flag
3. SSH to `aivcs@aivcs.local`, start tmux session
4. Run stages in remote session via SSH
5. Stream output back to local machine
6. Cache artifacts stored on remote
7. Developer can continue working locally

## Configuration

### .local-ci.toml on Remote

```toml
[cache]
# Where to store cache and build artifacts
skip_dirs = [".git", "target", ".github", "scripts", ".claude"]
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
```

### Autossh Systemd Service (Optional)

Create `/etc/systemd/user/local-ci-tunnel.service`:

```ini
[Unit]
Description=AutoSSH Tunnel for Remote CI on aivcs@aivcs.local
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=%u
Environment="AUTOSSH_GATETIME=0"
Environment="AUTOSSH_PORT=0"
ExecStart=/opt/homebrew/bin/autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
```

Enable and start:

```bash
systemctl --user enable local-ci-tunnel.service
systemctl --user start local-ci-tunnel.service
systemctl --user status local-ci-tunnel.service
```

## Implementation Plan

### Phase 1: Foundation (Foundation for remote support)

**Goal:** Enable remote stage execution infrastructure

1. **Add remote execution support to main.go:**
   - `--remote` flag: SSH host (e.g., `aivcs@aivcs.local`)
   - `--session` flag: tmux session name (default: project name + PID)
   - Detect if running remotely or locally

2. **Create `remote.go`:**
   - `RemoteExecutor` struct with SSH client
   - `runRemoteStage(stage, host, session)` function
   - Stream output from remote session back to local
   - Capture exit codes via sentinel files (like knittingCrab Issue #77)

3. **Update `main.go` stage execution:**
   - Check if `--remote` flag set
   - If remote: use RemoteExecutor
   - If local: use current execution path (ProcessExecutor)

### Phase 2: Caching & Artifacts

**Goal:** Synchronize cache and artifacts between local/remote

1. **Remote cache directory:**
   - `/tmp/local-ci-cache/<developer>/<project>`
   - Survives across sessions
   - Separate per developer to avoid conflicts

2. **Artifact sync (optional):**
   - Copy build artifacts back to local after success
   - Or store on NFS/shared storage (if available)

3. **Lock file handling:**
   - Remote machine maintains canonical lock file
   - Prevents concurrent builds on same project

### Phase 3: Session Management

**Goal:** Improve session lifecycle and debugging

1. **Session enumeration:**
   - `local-ci --remote aivcs@aivcs.local --list-sessions`
   - Show active sessions, PID, status

2. **Session cleanup:**
   - `local-ci --remote aivcs@aivcs.local --kill-session <name>`
   - Gracefully stop ongoing builds

3. **Logs & monitoring:**
   - `local-ci --remote aivcs@aivcs.local --logs <session>`
   - Tail build logs from remote session

## SSH Authentication

### Public Key Setup

```bash
# Generate key if needed (on local machine)
ssh-keygen -t ed25519 -f ~/.ssh/id_rsa_aivcs -C "local-ci@$(hostname)"

# Add public key to aivcs@aivcs.local
ssh-copy-id -i ~/.ssh/id_rsa_aivcs.pub aivcs@aivcs.local

# Configure SSH config (~/.ssh/config)
Host aivcs.local
    User aivcs
    IdentityFile ~/.ssh/id_rsa_aivcs
    ControlMaster auto
    ControlPath ~/.ssh/control-%h-%p-%r
    ControlPersist yes
```

Then use:
```bash
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
```

### SSH Agent (for passphrases)

```bash
# Start SSH agent
eval "$(ssh-agent -s)"

# Add key with passphrase
ssh-add ~/.ssh/id_rsa_aivcs
# Enter passphrase once, agent caches it
```

## Troubleshooting

### "command not found: autossh"

```bash
# On macOS
brew install autossh

# On Linux
sudo apt install autossh  # Ubuntu/Debian
sudo yum install autossh  # RHEL/CentOS
```

### "connection refused" or "connection timed out"

1. Verify `aivcs@aivcs.local` is reachable:
   ```bash
   ssh aivcs@aivcs.local echo "Connected"
   ```

2. Check SSH key authentication:
   ```bash
   ssh -v aivcs@aivcs.local  # verbose output
   ```

3. Verify tmux is installed on remote:
   ```bash
   ssh aivcs@aivcs.local which tmux
   ```

### Session already exists

If you restart and want to reuse the session:

```bash
# Attach to existing session
tmux attach-session -t onion

# Or create new session with different name
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion-2"
```

### Build fails on remote but works locally

1. Check Rust toolchain versions:
   ```bash
   ssh aivcs@aivcs.local rustc --version
   rustc --version  # local
   ```

2. Verify Nix cache is configured (if using flake.nix):
   ```bash
   ssh aivcs@aivcs.local nix flake update
   ```

3. Check available disk space on remote:
   ```bash
   ssh aivcs@aivcs.local df -h
   ```

## Related Concepts

### Comparison: local-ci vs knittingCrab Remote Execution

**knittingCrab (Issue #77):**
- Uses `ExecutionLocation::RemoteSession`
- Sentinel file strategy for exit codes
- `SshTmuxSessionExecutor` wraps commands
- Designed for task scheduling with resource allocation

**local-ci (proposed):**
- Uses `--remote` flag for one-time sessions
- Simpler: stream output, poll exit code
- Single command per invocation
- Focused on developer CI workflows

### Why SSH+tmux?

- **Persistence**: Builds continue if SSH connection drops (autossh reconnects)
- **Isolation**: Each developer gets own session
- **Monitoring**: Can `tmux capture-pane -p` to see output
- **Interactivity**: Developers can manually run follow-up commands
- **Simple**: No external queue/message bus needed

## Next Steps

1. **Prototype Phase 1:**
   - Implement `--remote` flag in main.go
   - Create `remote.go` with SSH execution
   - Test with single project

2. **Test matrix:**
   - Rust workspaces (1-8 crates)
   - TypeScript/Bun projects
   - Mixed type projects

3. **Documentation:**
   - Add `.local-ci-remote.toml` for remote-specific overrides
   - Document remote-specific features in README.md
   - Add troubleshooting for common SSH/tmux issues

4. **CI Integration:**
   - Run `local-ci --remote` in GitHub Actions matrix
   - Cache results in shared storage

## Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `remote.go` | Create | Remote SSH execution & output streaming |
| `main.go` | Modify | Add `--remote` and `--session` flags |
| `REMOTE_CI_SETUP.md` | Create | This document (setup & troubleshooting) |
| `.local-ci-remote.toml` | Create | Example remote-specific config |
| `README.md` | Modify | Add "Remote Execution" section |

## Example: Complete Workflow

```bash
# Terminal 1: Start autossh tunnel (one-time per session)
$ autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
# [SSH connects, tmux session created, prompt appears]

# Terminal 2: Run local-ci remotely (from project dir)
$ cd ~/engineering/code/my-rust-project
$ local-ci --remote aivcs@aivcs.local --session onion

# local-ci detects --remote, SSHes to aivcs@aivcs.local
# Runs stages in tmux session 'onion'
# Streams output to local terminal:
# 🚀 Running local CI pipeline...
#
# ::group::fmt
# $ cargo fmt --all -- --check
# ::endgroup::
# ✓ fmt (245ms)
#
# ... (clippy, test output)
#
# 📊 Summary:
#   Total stages: 3
#   Passed: 3
#   Total time: 8234ms

# Terminal 1: Manual intervention (if needed)
$ # Still in tmux session 'onion'
$ cargo build  # Run manual commands after CI
```

## Security Considerations

- **SSH keys**: Use key-based auth only, no passwords
- **Key restrictions**: Limit SSH key to specific commands if possible
- **Firewall**: Ensure `aivcs@aivcs.local` is only accessible from trusted networks
- **Auditing**: Monitor SSH connection logs: `ssh aivcs@aivcs.local "tail -f /var/log/auth.log"`

## Performance Notes

- **First run**: ~5-10 minutes (downloads dependencies, builds)
- **Cached run**: ~10-20 seconds (cache hit on all stages)
- **Network latency**: SSH adds ~50-100ms per command
- **Nix cache**: If configured, saves 10+ minutes on flake.nix rebuilds
