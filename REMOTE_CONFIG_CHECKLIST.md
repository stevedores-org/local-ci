# Remote CI Configuration Checklist

## Overview

This checklist covers everything needed to set up local-ci as remote-ci on aivcs@aivcs.local (Mac Studio) with SSH+tmux persistence.

## Pre-requisites: Local Machine Setup

### 1. Install autossh (local machine)

```bash
# macOS
brew install autossh

# Linux (Debian/Ubuntu)
sudo apt install autossh

# Linux (RHEL/CentOS)
sudo yum install autossh

# Verify installation
which autossh
autossh -h
```

**Status:** ☐ Installed

### 2. SSH Key Authentication

```bash
# Generate ed25519 key (if needed)
ssh-keygen -t ed25519 -f ~/.ssh/id_rsa_aivcs -C "local-ci"

# Copy to remote
ssh-copy-id -i ~/.ssh/id_rsa_aivcs.pub aivcs@aivcs.local

# Test connection
ssh aivcs@aivcs.local echo "Connected"
```

**Status:** ☐ Key installed & authentication working

### 3. SSH Config (~/.ssh/config)

Add or update:

```
Host aivcs.local
    User aivcs
    IdentityFile ~/.ssh/id_rsa_aivcs
    ControlMaster auto
    ControlPath ~/.ssh/control-%h-%p-%r
    ControlPersist yes
```

**Status:** ☐ SSH config created

### 4. SSH Agent (optional but recommended)

```bash
# Start agent (add to ~/.zshrc or ~/.bashrc)
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_rsa_aivcs
```

**Status:** ☐ SSH agent configured

---

## Remote Machine Setup (aivcs@aivcs.local)

### 5. Install Go (if not already installed)

```bash
# On aivcs@aivcs.local
go version  # Check if already installed

# If missing, download and install
# https://golang.org/dl/ (Go 1.22+)
```

**Status:** ☐ Go 1.22+ available on remote

### 6. Build and Install local-ci Binary

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local

cd /tmp
git clone https://github.com/stevedores-org/local-ci
cd local-ci
go build -o /usr/local/bin/local-ci
chmod +x /usr/local/bin/local-ci

# Verify
local-ci --version
```

**Status:** ☐ local-ci binary installed to /usr/local/bin/local-ci

### 7. Verify Rust Toolchain (if running Rust projects)

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local

rustc --version
cargo --version
rustup update
```

**Status:** ☐ Rust toolchain up-to-date

### 8. Verify tmux Installation

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local which tmux

# If not installed:
# macOS: brew install tmux
# Linux: sudo apt install tmux (Debian) or sudo yum install tmux (RHEL)
```

**Status:** ☐ tmux available on remote

### 9. Create Remote Cache Directory

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local

mkdir -p /tmp/local-ci-cache
chmod 755 /tmp/local-ci-cache

# Optional: per-developer subdirectories for isolation
mkdir -p /tmp/local-ci-cache/$USER
```

**Status:** ☐ Remote cache directory created

### 10. Configure Nix Cache (if using Nix)

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local

# For stevedores-org Nix cache
cat >> ~/.config/nix/nix.conf <<EOF
extra-substituters = https://nix-cache.stevedores.org https://cache.nixos.org
trusted-public-keys = oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI= cache.nixos.org-1:6NCHdD59X431o0gWypQydGvjwydGG2UZTvhjGJNsx6E=
EOF

# Test cache configuration
nix flake update  # or similar Nix operation
```

**Status:** ☐ Nix cache configured (if applicable)

---

## Project Configuration

### 11. Create .local-ci.toml in Project

For each project that will run on remote:

```bash
cd /path/to/rust-project

# If not already done
local-ci init

# This creates .local-ci.toml with defaults
```

**Status:** ☐ .local-ci.toml exists in project

### 12. Optional: Create .local-ci-remote.toml

For remote-specific overrides (longer timeouts, disabled stages, etc.):

```bash
# Copy example
cp /tmp/local-ci/.local-ci-remote.toml.example .local-ci-remote.toml

# Edit as needed
vim .local-ci-remote.toml
```

**Status:** ☐ .local-ci-remote.toml configured (optional)

### 13. Verify Project Builds Locally

Before testing remote execution:

```bash
local-ci
# Ensure all stages pass locally
```

**Status:** ☐ Project builds successfully locally

---

## Execution & Testing

### 14. Create autossh Tunnel

```bash
# On local machine, in a persistent terminal
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"

# You should see a tmux prompt
# Leave this terminal running
```

**Status:** ☐ autossh tunnel active

### 15. Test SSH Access & tmux Session

In a new terminal:

```bash
# Verify you can access the remote tmux session
ssh aivcs@aivcs.local tmux list-sessions

# Should show: onion (1 windows) [...]
```

**Status:** ☐ Remote tmux session accessible

### 16. Test Manual Build on Remote

```bash
# Still in the tmux session (first terminal)
cd /path/to/rust-project

local-ci fmt clippy

# Should see output formatted like GitHub Actions
# ✓ fmt (200ms)
# ✓ clippy (1234ms)
```

**Status:** ☐ Manual build works on remote

### 17. Test Full Pipeline

```bash
# Full build with test
local-ci

# Should complete all stages
# Check cache hit percentage
```

**Status:** ☐ Full pipeline works on remote

---

## Optional: Systemd Service (Persistent Tunnel)

### 18. Create systemd User Service

```bash
# On local machine
cat > ~/.config/systemd/user/local-ci-tunnel.service <<EOF
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
EOF

# Create directory if needed
mkdir -p ~/.config/systemd/user

# Reload systemd and enable service
systemctl --user daemon-reload
systemctl --user enable local-ci-tunnel.service
systemctl --user start local-ci-tunnel.service

# Check status
systemctl --user status local-ci-tunnel.service
```

**Status:** ☐ Systemd service created & enabled

### 19. Verify Service Auto-start

```bash
# Simulate system restart
systemctl --user restart local-ci-tunnel.service

# Verify tunnel is still active
ssh aivcs@aivcs.local tmux list-sessions
```

**Status:** ☐ Service auto-reconnects after restart

---

## Performance Tuning

### 20. Optimize SSH Connection

Add to ~/.ssh/config:

```
Host aivcs.local
    # Reuse connections for faster subsequent commands
    ControlMaster auto
    ControlPath ~/.ssh/control-%h-%p-%r
    ControlPersist 600  # Keep open for 10 minutes after last command

    # Compression (useful for slower networks)
    # Compression yes

    # Use faster key exchange
    KexAlgorithms curve25519-sha256,diffie-hellman-group16-sha512
    Ciphers chacha20-poly1305@openssh.com,aes-256-gcm@openssh.com
```

**Status:** ☐ SSH connection optimized

### 21. Monitor Disk Space (Remote)

```bash
# On aivcs@aivcs.local
ssh aivcs@aivcs.local

df -h /tmp  # Check cache directory
du -sh /tmp/local-ci-cache  # Check cache size

# If cache grows too large
rm -rf /tmp/local-ci-cache/*
```

**Status:** ☐ Disk space monitoring plan in place

---

## Troubleshooting

### Common Issues & Solutions

| Issue | Solution | Status |
|-------|----------|--------|
| `autossh: command not found` | Reinstall: `brew install autossh` | ☐ |
| `Connection timed out` | Check network, firewall; verify `aivcs.local` is reachable | ☐ |
| `tmux: command not found` | Install on remote: `brew install tmux` | ☐ |
| `Session already exists` | Use different session name or `tmux kill-session -t onion` | ☐ |
| `local-ci: command not found` (remote) | Reinstall: `go build -o /usr/local/bin/local-ci` | ☐ |
| `Rust compilation fails` | Check Rust version: `rustc --version` on both machines | ☐ |
| `Permission denied (publickey)` | Ensure SSH key is in agent: `ssh-add ~/.ssh/id_rsa_aivcs` | ☐ |
| `Out of disk space` | Clear cache: `rm -rf /tmp/local-ci-cache` | ☐ |

---

## Sign-Off

When all items are checked, you can use:

```bash
# Quick test all stages are working
cd ~/engineering/code/my-rust-project
local-ci
echo "✅ Remote CI setup complete!"
```

**Overall Status:** ☐ All items complete

**Date completed:** _____________

**Notes:** _____________________________________________

---

## Quick Reference

### Start remote session
```bash
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
```

### Run CI remotely (in tmux session)
```bash
local-ci              # All stages
local-ci fmt clippy   # Specific stages
local-ci --fix        # Auto-fix formatting
local-ci --no-cache   # Skip cache
```

### Monitor or reconnect to session
```bash
ssh aivcs@aivcs.local tmux attach-session -t onion
tmux capture-pane -t onion -p  # View output without attaching
```

### Kill session (cleanup)
```bash
ssh aivcs@aivcs.local tmux kill-session -t onion
```
