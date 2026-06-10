# SSH identity (codified)

Unified SSH logins for **local-ci remote execution** and agent automation.

**All fleet nodes live in Tailscale only** — use MagicDNS names from `tailscale status`, never `.local` mDNS or LAN IPs.

## In Tailscale

Source of truth: `tailscale status` (names in the **DNS** column).

| Tailscale IP | DNS name | OS | Preset | SSH user |
|--------------|----------|-----|--------|----------|
| `100.84.125.48` | **`downhome`** | macOS | `downhome` / `studio` | `aivcs` |
| `100.81.115.15` | `uranus` | macOS | `uranus` | `aivcs` |
| `100.103.163.28` | `discovery` | macOS | `discovery` | `aivcs` |
| `100.124.89.47` | `spark-bde7` | linux | `sparky` / `aivcs2` | `aivcs2` |
| `100.85.43.19` | `msi` | windows | `msi` | `aivcs` |

```bash
tailscale status
tailscale ping msi      # reachability over tailnet
tailscale ping downhome
```

**Local machine:** you are on **`downhome`** (Apple Silicon Mac, user `aivcs`). Run `local-ci` here; fan out with `--remote-host <preset>`.

**`msi`:** online when logged into Windows + Tailscale; offline when logged out.

## Fleet (presets)
| Preset | In Tailscale (DNS) | OS | GPU | User |
|--------|-------------------|-----|-----|------|
| `downhome` | `downhome` | macOS (Apple Silicon) | — | `aivcs` |
| `uranus` | `uranus` | macOS | — | `aivcs` |
| `discovery` | `discovery` | macOS | — | `aivcs` |
| `sparky` | `spark-bde7` | Linux | Blackwell (DGX) | `aivcs2` |
| `msi` | `msi` | Windows 11 | RTX 5070 | `aivcs` |

Alias: `studio` → `downhome`.

## Policy

| Platform | Canonical user | Scope |
|----------|----------------|-------|
| **macOS** (tailnet) | `aivcs` | `downhome`, `uranus`, `discovery`, all macOS nodes |
| **Linux — DGX Spark** | `aivcs2` | `spark-bde7` only (`sparky` preset) |
| **Windows 11** | `aivcs` | `msi` only (`msi` preset) |

**Rules**

1. **Automation uses canonical users only** — agents, `local-ci --remote*`, and CI babysit scripts use the users above.
2. **`stevenirvin` is not an automation account** — personal/interactive use only.
3. **In Tailscale only** — preset `host` must match the **DNS name** from `tailscale status` (`downhome`, `uranus`, `msi`, …). No `.local`, no raw LAN IPs in presets.
4. **Explicit `user@host` overrides** — if a preset or `--remote` includes `@`, that user wins (escape hatch only).

### Windows (`msi`) caveats

- **Tailscale:** offline while logged out; online when Windows session is active (check `tailscale ping msi`).
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
| `downhome` | `macos` | `aivcs@downhome` |
| `uranus` | `macos` (default) | `aivcs@uranus` |
| `discovery` | `macos` | `aivcs@discovery` |
| `spark-bde7` | `linux_spark` | `aivcs2@spark-bde7` |
| `msi` | `windows` | `aivcs@msi` |

## Operator checklist

```bash
tailscale status    # downhome = this Mac; msi should show online when logged in
tailscale ping msi
tailscale ping uranus

ssh aivcs@downhome echo ok   # local tailnet name (same user as remote macOS nodes)
ssh aivcs@uranus echo ok
ssh aivcs2@spark-bde7 echo ok
ssh aivcs@msi echo ok        # OpenSSH Server on Windows 11

local-ci                        # run locally on downhome
local-ci --remote-host uranus --dry-run
local-ci --remote-host sparky fmt clippy test
local-ci --remote-host msi test   # WSL + tmux if using full remote path
```

## Adding a node

1. Add `[hosts.<preset>]` with bare Tailscale name and correct `platform` (`macos`, `linux_spark`, `windows`).
2. Install the **canonical** user's SSH public key on the remote.
3. Enable Remote Login (macOS), `sshd` (Linux), or OpenSSH Server (Windows).
4. Run `local-ci --remote-host <preset> --dry-run` before heavy stages.

## Related

- [REMOTE_CI_SETUP.md](../REMOTE_CI_SETUP.md) — SSH+tmux workflow
- [.local-ci-remote.toml.example](../.local-ci-remote.toml.example) — ship-ready presets
