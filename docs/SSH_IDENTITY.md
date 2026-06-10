# SSH identity (codified)

Unified SSH logins for **local-ci remote execution** and agent automation over Tailscale.

## Policy

| Platform | Canonical user | Scope |
|----------|----------------|-------|
| **macOS** (tailnet) | `aivcs` | `uranus`, `discovery`, `downhome`, all other macOS build/review nodes |
| **Linux — DGX Spark** | `aivcs2` | `spark-bde7` only (`sparky` preset) |

**Rules**

1. **Automation uses canonical users only** — agents, `local-ci --remote*`, and CI babysit scripts SSH as `aivcs` (macOS) or `aivcs2` (Spark).
2. **`stevenirvin` is not an automation account** — personal/interactive use only; do not put it in `.local-ci-remote.toml` or agent skills.
3. **Tailscale MagicDNS** — host field is the tailnet name (`uranus`, `spark-bde7`), not `.local` mDNS or LAN IPs.
4. **Explicit `user@host` overrides** — if a preset or `--remote` includes `@`, that user wins (escape hatch only).

## Defaults in config

`.local-ci-remote.toml` centralizes users under `[ssh_defaults]`:

```toml
[ssh_defaults]
macos_user = "aivcs"
linux_spark_user = "aivcs2"
```

Bare host names expand at runtime:

| Config `host` | `platform` | Resolved SSH target |
|---------------|------------|---------------------|
| `uranus` | `macos` (default) | `aivcs@uranus` |
| `discovery` | `macos` | `aivcs@discovery` |
| `spark-bde7` | `linux_spark` | `aivcs2@spark-bde7` |

## Operator checklist

```bash
tailscale status
tailscale ping uranus

# Canonical smoke tests
ssh aivcs@uranus echo ok
ssh aivcs2@spark-bde7 echo ok

local-ci --remote-host uranus --dry-run
local-ci --remote-host sparky fmt clippy test
```

## Adding a node

1. Add `[hosts.<preset>]` with bare Tailscale name and correct `platform`.
2. Install the **canonical** user's SSH public key on the remote.
3. Enable Remote Login (macOS) or `sshd` (Linux).
4. Run `local-ci --remote-host <preset> --dry-run` before heavy stages.

## Related

- [REMOTE_CI_SETUP.md](../REMOTE_CI_SETUP.md) — SSH+tmux workflow
- [.local-ci-remote.toml.example](../.local-ci-remote.toml.example) — ship-ready presets
