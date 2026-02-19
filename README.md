# workiq-proxy

MCP compatibility proxy for [Microsoft Work IQ](https://github.com/microsoft/work-iq-mcp). Wraps the `workiq mcp` server and fixes interoperability issues with Claude Code, VS Code, and other MCP clients.

## What it does

Work IQ's MCP server only declares `tools` and `logging` capabilities. MCP clients like Claude Code also call `prompts/list` and `resources/list`, which return `-32601` errors and can break the connection ([work-iq-mcp#38](https://github.com/microsoft/work-iq-mcp/issues/38)).

The proxy sits between the MCP client and the Work IQ server:

```
MCP Client <-stdio-> workiq-proxy <-stdio-> workiq mcp
```

It:

1. **Intercepts unsupported methods** — Returns empty valid responses for `prompts/list`, `resources/list`, and `resources/templates/list`
2. **Patches initialize capabilities** — Adds `prompts` and `resources` so clients see a fully capable server
3. **Self-disables** — If a future Work IQ version declares these capabilities, the proxy stops intercepting
4. **Auth pre-flight** — Checks for cached auth tokens before forwarding `ask_work_iq`, returning an actionable error instead of letting the binary block stdio with a browser flow
5. **Error enrichment** — Wraps opaque errors like "Failed to create conversation" with troubleshooting context
6. **Synthetic tools** — Exposes 7 domain-specific tools (`search_emails`, `search_documents`, `search_chats`, `search_channels`, `search_meetings`, `search_people`, `search_external`) backed by `ask_work_iq`
7. **Traffic logging** — Logs all JSON-RPC messages to `~/.work-iq-cli/mcp-traffic.log`

## Install

### Via npm (recommended)

```bash
npm install -g @stuffbucket/workiq-proxy
```

### Direct binary

Download from [GitHub Releases](https://github.com/stuffbucket/workiq-proxy/releases) and place in your PATH.

## Prerequisites

Before using the proxy, authenticate Work IQ from your terminal:

```bash
# Accept the EULA
npx -y @microsoft/workiq accept-eula

# Trigger browser auth and cache tokens
npx -y @microsoft/workiq ask -q "What's on my calendar?"
```

## Configure

### Claude Code

```bash
claude mcp add --transport stdio workiq --scope project -- npx -y @stuffbucket/workiq-proxy
```

Or add to `.mcp.json`:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@stuffbucket/workiq-proxy"]
    }
  }
}
```

### VS Code

Add to your MCP settings:

```json
{
  "workiq": {
    "command": "npx",
    "args": ["-y", "@stuffbucket/workiq-proxy"],
    "tools": ["*"]
  }
}
```

### Direct binary

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "/path/to/workiq-proxy"
    }
  }
}
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--workiq-cmd` | `npx -y @microsoft/workiq mcp` | Command to spawn the Work IQ MCP server |
| `--log-file` | `~/.work-iq-cli/mcp-traffic.log` | Override log file path |
| `--no-log` | `false` | Disable traffic logging |

Example with a globally installed binary:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "workiq-proxy",
      "args": ["--workiq-cmd", "workiq mcp"]
    }
  }
}
```

## Verify

After configuring, check that the server connects:

- **Claude Code:** Run `/mcp` to see server status and available tools
- **VS Code:** Check the MCP server status in the output panel

You should see 9 tools: `accept_eula`, `ask_work_iq`, plus the 7 synthetic search tools.

## Troubleshoot

**"No cached auth token found"** — Run `workiq ask -q "What's on my calendar?"` in your terminal to authenticate.

**"Failed to create conversation"** — Your auth token expired or your Microsoft 365 Copilot license is not active. Re-authenticate from the terminal.

**Server not connecting** — Check `~/.work-iq-cli/mcp-traffic.log` for the full JSON-RPC transcript.

## Development

```bash
# Build
make build

# Test
make test

# Cross-compile all platforms
make all
```

## License

MIT
