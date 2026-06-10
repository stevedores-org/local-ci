# SSH identity (codified)

Unified SSH logins for **local-ci remote execution** and agent automation over Tailscale.

## Fleet

**4 remote compute nodes** (+ operator Mac `downhome`):

| Preset | Tailscale | OS | GPU | User |
|--------|-----------|-----|-----|------|
| `uranus` | `uranus` | macOS | — | `aivcs` |
| `discovery` | `discovery` | macOS | — | `aivcs` |
| `sparky` | `spark-bde7` | Linux | Blackwell (DGX) | `aivcs2` |
| `msi` | `msi` | **Windows 11** | **RTX 5070** | `aivcs` |

| Preset | Tailscale | Role |
|--------|-----------|------|
| `studio` | `downhome` | Operator machine (runs `local-ci` locally) |

`msi` shows **offline** in `tailscale status` when logged out (last seen ~2d ago until it reconnects).

## Policy

| Platform | Canonical user | Scope |
|----------|----------------|-------|
| **macOS** (tailnet) | `aivcs` | `uranus`, `discovery`, `downhome`, all macOS nodes |
| **Linux — DGX Spark** | `aivcs2` | `spark-bde7` only (`sparky` preset) |
| **Windows 11** | `aivcs` | `msi` only (`msi` preset) |

**Rules**

1. **Automation uses canonical users only** — agents, `local-ci --remote*`, and CI babysit scripts use the users above.
2. **`stevenirvin` is not an automation account** — personal/interactive use only.
3. **Tailscale MagicDNS** — preset `host` is the tailnet name (`uranus`, `msi`, …), not `.local` mDNS or LAN IPs.
4. **Explicit `user@host` overrides** — if a preset or `--remote` includes `@`, that user wins (escape hatch only).

### Windows (`msi`) caveats

- **Tailscale:** node is unreachable while logged out / offline.
- **OpenSSH Server** must be enabled on Windows 11 (Settings → Apps → Optional features → OpenSSH Server).
- **local-ci remote path** uses SSH + **tmux** today — native Windows has no tmux. For GPU builds on msi, use **WSL2** with tmux inside WSL, or run checks locally until native Windows remote support lands.

## Defaults in config

```toml
[ssh_defaults]
macos_user = "aivcs"
linux_spark_user = "aivcs2"
windows_user = "aivcs"
```

Bare host names expand at runtime:

| Config `host` | `platform` | Resolved SSH target |
|---------------|------------|---------------------|
| `uranus` | `macos` (default) | `aivcs@uranus` |
| `discovery` | `macos` | `aivcs@discovery` |
| `spark-bde7` | `linux_spark` | `aivcs2@spark-bde7` |
| `msi` | `windows` | `aivcs@msi` |

## Operator checklist

```bash
tailscale status
tailscale ping uranus
tailscale ping msi    # fails while msi is offline

ssh aivcs@uranus echo ok
ssh aivcs2@spark-bde7 echo ok
ssh aivcs@msi echo ok   # when msi is online + OpenSSH enabled

local-ci --remote-host uranus --dry-run
local-ci --remote-host sparky fmt clippy test
```

## Adding a node

1. Add `[hosts.<preset>]` with bare Tailscale name and correct `platform` (`macos`, `linux_spark`, `windows`).
2. Install the **canonical** user's SSH public key on the remote.
3. Enable Remote Login (macOS), `sshd` (Linux), or OpenSSH Server (Windows).
4. Run `local-ci --remote-host <preset> --dry-run` before heavy stages.

## Related

- [REMOTE_CI_SETUP.md](../REMOTE_CI_SETUP.md) — SSH+tmux workflow
- [.local-ci-remote.toml.example](../.local-ci-remote.toml.example) — ship-ready presets
