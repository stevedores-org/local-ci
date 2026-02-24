# Local-CI Remote Setup — Execution Plan

**Target:** aivcs@aivcs.local (Mac Studio)
**Purpose:** Run expensive builds on remote hardware via SSH+tmux
**Status:** Ready to execute (requires Mac Studio to be online)

---

## Prerequisites Check

### On Your Local Machine

```bash
# 1. Verify autossh is installed
which autossh
autossh -h

# 2. Verify SSH key exists
ls -la ~/.ssh/id_rsa_aivcs.pub

# 3. Test SSH connection (once Mac Studio is online)
ssh aivcs@aivcs.local "uname -a"
```

**If Mac Studio is NOT currently reachable:**
- [ ] Power on Mac Studio
- [ ] Verify it's on the same network as your local machine
- [ ] Check if mDNS is working: `nslookup aivcs.local` or `avahi-resolve -n aivcs.local`
- [ ] Or use IP address instead: Get IP from network settings/router

---

## Phase 1: One-Time Remote Setup

Execute these commands **on aivcs.local** (Mac Studio):

### Step 1A: Install Go (if needed)

```bash
ssh aivcs@aivcs.local

# Check if Go is installed
go version

# If not installed, download from https://golang.org/dl/
# (Go 1.22+)
```

**Status:** ☐ Complete

### Step 1B: Build and Install local-ci Binary

```bash
ssh aivcs@aivcs.local

# Clone repo
cd /tmp
rm -rf local-ci  # Clean up if exists
git clone https://github.com/stevedores-org/local-ci
cd local-ci

# Build
go build -o local-ci

# Install to system binary directory
sudo cp local-ci /usr/local/bin/
sudo chmod +x /usr/local/bin/local-ci

# Verify
local-ci --version
```

**Status:** ☐ Complete

### Step 1C: Verify Rust Toolchain

```bash
ssh aivcs@aivcs.local

# Check versions
rustc --version
cargo --version

# Update if needed
rustup update
```

**Status:** ☐ Complete

### Step 1D: Verify tmux is Installed

```bash
ssh aivcs@aivcs.local

which tmux
tmux -V

# If not installed:
# brew install tmux
```

**Status:** ☐ Complete

### Step 1E: Create Cache Directory

```bash
ssh aivcs@aivcs.local

mkdir -p /tmp/local-ci-cache
chmod 755 /tmp/local-ci-cache
ls -la /tmp/local-ci-cache
```

**Status:** ☐ Complete

### Step 1F: Configure Nix Cache (Optional)

```bash
ssh aivcs@aivcs.local

# Only if you use Nix/flake.nix
cat >> ~/.config/nix/nix.conf <<EOF
extra-substituters = https://nix-cache.stevedores.org https://cache.nixos.org
trusted-public-keys = oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI= cache.nixos.org-1:6NCHdD59X431o0gWypQydGvjwydGG2UZTvhjGJNsx6E=
EOF

# Test
nix flake update  # or similar
```

**Status:** ☐ Complete (optional)

---

## Phase 2: Create Persistent SSH Tunnel

Execute on your **local machine**:

### Step 2A: Establish autossh Tunnel

```bash
# Open a new terminal and keep it running
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
```

**Expected output:**
```
[tmux session prompt appears]
aivcs@mac-studio ~> _
```

**Status:** ☐ Tunnel established

### Step 2B: Verify Connection (in new terminal)

```bash
# In another terminal, verify the remote tmux session exists
ssh aivcs@aivcs.local tmux list-sessions

# Expected: onion (1 windows) [...]
```

**Status:** ☐ Verified

---

## Phase 3: Test Single Project Build

### Step 3A: Clone a Test Project

```bash
# In the tmux session (terminal from Step 2A)
cd /tmp
git clone https://github.com/stevedores-org/knittingCrab
cd knittingCrab
```

**Status:** ☐ Project cloned

### Step 3B: Run Single Stage

```bash
# Still in tmux session
local-ci fmt

# Should see output like:
# 🚀 Running local CI pipeline...
# ::group::fmt
# $ cargo fmt --all -- --check
# ::endgroup::
# ✓ fmt (245ms)
```

**Status:** ☐ Single stage works

### Step 3C: Run Full Pipeline

```bash
# Full build with all stages
local-ci

# Should see:
# ✓ fmt (245ms)
# ✓ clippy (2345ms)
# ✓ test (5678ms)
# 📊 Summary: ...
```

**Status:** ☐ Full pipeline works

### Step 3D: Test Cache Hit (Second Run)

```bash
# Run again - should be much faster
time local-ci

# Should show cache hits:
# 📊 Summary:
#   Cached: 3 (100%)
#   Total time: ~50ms
```

**Status:** ☐ Caching works

---

## Phase 4: Test Your Own Project

### Step 4A: Copy Project to Remote

```bash
# Option 1: Git clone
cd /tmp
git clone /path/to/your-rust-project

# Option 2: Use git if already on remote
cd /tmp
git clone https://github.com/your-org/your-project
```

**Status:** ☐ Project available on remote

### Step 4B: Create .local-ci.toml (if needed)

```bash
cd /tmp/your-rust-project

# Check if .local-ci.toml exists
ls -la .local-ci.toml

# If not, initialize
local-ci init

# Review and adjust timeouts if needed
cat .local-ci.toml
```

**Status:** ☐ Config created

### Step 4C: Run CI Pipeline

```bash
# In tmux session
cd /tmp/your-rust-project
local-ci

# Monitor output and timing
```

**Status:** ☐ Your project builds

---

## Phase 5: Optional — Persistent Tunnel Service

Make the tunnel auto-start on system login (macOS):

### Step 5A: Create systemd User Service (Linux)

```bash
# On local machine (if Linux)
mkdir -p ~/.config/systemd/user

cat > ~/.config/systemd/user/local-ci-tunnel.service <<'EOF'
[Unit]
Description=AutoSSH Tunnel to aivcs.local
After=network.target

[Service]
Type=simple
User=%u
ExecStart=/opt/homebrew/bin/autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable local-ci-tunnel
systemctl --user start local-ci-tunnel
systemctl --user status local-ci-tunnel
```

**Status:** ☐ Service created (optional)

### Step 5B: Create LaunchAgent (macOS)

```bash
# On local machine (if macOS)
mkdir -p ~/Library/LaunchAgents

cat > ~/Library/LaunchAgents/com.stevedores.local-ci-tunnel.plist <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.stevedores.local-ci-tunnel</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/autossh</string>
    <string>-M</string>
    <string>0</string>
    <string>-t</string>
    <string>aivcs@aivcs.local</string>
    <string>/opt/homebrew/bin/tmux new-session -A -s onion</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/local-ci-tunnel.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/local-ci-tunnel.err</string>
</dict>
</plist>
EOF

launchctl load ~/Library/LaunchAgents/com.stevedores.local-ci-tunnel.plist
launchctl start com.stevedores.local-ci-tunnel

# Verify
launchctl list | grep local-ci-tunnel
```

**Status:** ☐ LaunchAgent installed (optional)

---

## Verification Checklist

Before declaring setup complete, verify:

- [ ] aivcs.local is reachable and online
- [ ] SSH key authentication works (`ssh aivcs@aivcs.local` succeeds)
- [ ] Go 1.22+ installed on remote
- [ ] local-ci binary installed to `/usr/local/bin/local-ci`
- [ ] Rust toolchain up-to-date
- [ ] tmux installed and working
- [ ] Cache directory exists at `/tmp/local-ci-cache`
- [ ] autossh tunnel established and persistent
- [ ] Test project (knittingCrab) builds successfully
- [ ] Cache hits on second run
- [ ] Your own project builds successfully

---

## Quick Reference

### Start/Connect to Tunnel

```bash
# Start tunnel (first terminal)
autossh -M 0 -t aivcs@aivcs.local "/opt/homebrew/bin/tmux new-session -A -s onion"

# Connect to session (in another terminal)
ssh aivcs@aivcs.local tmux attach-session -t onion

# Or directly run commands in session
ssh aivcs@aivcs.local tmux send-keys -t onion "cd /tmp/project && local-ci" Enter
```

### Monitor/Debug

```bash
# View session output without attaching
ssh aivcs@aivcs.local tmux capture-pane -t onion -p

# List all sessions
ssh aivcs@aivcs.local tmux list-sessions

# Kill session (cleanup)
ssh aivcs@aivcs.local tmux kill-session -t onion
```

### Common Issues

| Issue | Solution |
|-------|----------|
| `autossh: command not found` | `brew install autossh` |
| `aivcs.local` not found | Power on Mac Studio, check network |
| `local-ci: command not found` | Run installation steps above |
| Session already exists | Use different name: `-s onion-2` |
| Permission denied (publickey) | Run `ssh-add ~/.ssh/id_rsa_aivcs` |

---

## Success Criteria

✅ **Setup is complete when:**
1. `ssh aivcs@aivcs.local local-ci --version` returns version
2. `autossh -M 0 -t aivcs@aivcs.local tmux list-sessions` shows active session
3. Full `local-ci` pipeline completes in tmux session
4. Build times show ~5-10 minutes for clean build, <1 minute for cached
5. Multiple projects can run independently in the same session

---

## Next Steps (After Setup)

Once remote execution is working, consider:

1. **Implement remote flag in local-ci:**
   - `local-ci --remote aivcs@aivcs.local`
   - Auto-detect remote execution, stream output back

2. **Create remote-specific overrides:**
   - `.local-ci-remote.toml` for longer timeouts
   - Disable slow/redundant stages on remote

3. **Integrate with CI/CD:**
   - Use remote execution in GitHub Actions
   - Share cache across developers

4. **Monitor resource usage:**
   - Check disk space on remote
   - Monitor CPU/memory during builds
   - Clean up old cache periodically

---

## Questions?

Refer to:
- `REMOTE_CI_SETUP.md` — Architecture & detailed explanations
- `REMOTE_CONFIG_CHECKLIST.md` — Verification items
- `.local-ci-remote.toml.example` — Config overrides

---

**Status:** Ready to execute
**Date:** 2026-02-24
