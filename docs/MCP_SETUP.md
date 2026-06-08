# local-ci MCP — local IDE setup

`local-ci serve` exposes CI stages over MCP stdio (`run_stage`, `run_all`, `get_stages`, etc.).

## Prerequisites

```bash
brew install go
go build -o local-ci .
# or: go install github.com/stevedores-org/local-ci@latest
```

## Server command

Use an **absolute path** to the binary in MCP configs:

```json
{
  "mcpServers": {
    "local-ci": {
      "command": "/absolute/path/to/local-ci",
      "args": ["serve"],
      "cwd": "/absolute/path/to/your/project"
    }
  }
}
```

`cwd` must be the project root containing `.local-ci.toml`.

## Tool-specific paths

| Tool | Config file |
|------|-------------|
| **Cursor** | `~/.cursor/mcp.json` or `<workspace>/.cursor/mcp.json` |
| **VS Code** (Copilot MCP) | `<workspace>/.vscode/mcp.json` |
| **Claude Desktop** | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) |
| **Windsurf** | `~/.codeium/windsurf/mcp_config.json` |

Restart the IDE (or refresh MCP in Windsurf Cascade) after editing.

## Fleet MCP hub

For Lornu AI shared state and provider tools, use [lornu-ai/lornu.ai-mcp](https://github.com/lornu-ai/lornu.ai-mcp) — run `./scripts/setup-mcp-local.sh` from that repo.
