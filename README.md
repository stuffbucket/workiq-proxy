# workiq-proxy

MCP compatibility proxy for [Microsoft Work IQ](https://github.com/microsoft/work-iq-mcp). Wraps the `workiq mcp` server and fixes interoperability issues with Claude Code, VS Code, and other MCP clients. Also provides an interactive REPL and an OpenAI-compatible HTTP API.

## What it does

Work IQ's MCP server only declares `tools` and `logging` capabilities. MCP clients like Claude Code also call `prompts/list` and `resources/list`, which return `-32601` errors and can break the connection ([work-iq-mcp#38](https://github.com/microsoft/work-iq-mcp/issues/38)).

In MCP mode the proxy sits between the MCP client and the Work IQ server:

```
MCP Client <-stdio-> workiq-proxy <-stdio-> workiq mcp
```

It:

1. **Intercepts unsupported methods** — Returns empty valid responses for `prompts/list`, `resources/list`, and `resources/templates/list`
2. **Patches initialize capabilities** — Adds `prompts` and `resources` so clients see a fully capable server
3. **Self-disables** — If a future Work IQ version declares these capabilities, the proxy stops intercepting
4. **Error enrichment** — Wraps opaque errors with troubleshooting context (EULA not accepted, token protection / error 530084, Entra ID conditional access / AADSTS, interactive login required, and "Failed to create conversation")
5. **Synthetic tools** — Exposes 7 domain-specific tools (`search_emails`, `search_documents`, `search_chats`, `search_channels`, `search_meetings`, `search_people`, `search_external`) backed by `ask_work_iq`
6. **Interactive REPL** — When run from a terminal with no arguments, launches a Bubble Tea UI with slash commands (`/ask`, `/emails`, `/docs`, `/chats`, `/channels`, `/meetings`, `/people`, `/accept-eula`, `/tools`, `/help`, `/quit`), session history, and glamour markdown rendering
7. **OpenAI-compatible HTTP API** — `workiq-proxy serve` starts a local server with `/v1/chat/completions` (streaming and non-streaming) and `/v1/models`
8. **CLI passthrough** — When run from an interactive terminal with arguments, delegates to the underlying `workiq` CLI (EULA acceptance, auth, queries) so you only need one package
9. **Traffic logging** — Logs all JSON-RPC messages to `~/.work-iq-cli/mcp-traffic.log`

## Install

```bash
npm install -g @stuffbucket/workiq-proxy
```

This also installs `@microsoft/workiq` as a dependency — no separate setup needed. The npm package provides both `workiq-proxy` and `wiq` as binaries.

Or download a binary from [GitHub Releases](https://github.com/stuffbucket/workiq-proxy/releases) and place it in your PATH.

## Prerequisites

Before using the proxy, set up Work IQ from your terminal:

```bash
# First run: accepts EULA interactively, then triggers browser auth
npx -y @stuffbucket/workiq-proxy ask
```

The interactive session walks you through EULA acceptance and signs you in via the browser. Once complete, the MCP server reuses cached tokens silently.

You can also accept the EULA directly:

```bash
npx -y @stuffbucket/workiq-proxy accept-eula
```

To verify auth works non-interactively:

```bash
npx -y @stuffbucket/workiq-proxy ask -q "What's on my calendar?"
```

## Commands

| Command | Description |
|---------|-------------|
| *(no args, TTY)* | Start the interactive REPL |
| `ask -q "..."` | Ask Microsoft 365 Copilot a question (passthrough to `workiq`) |
| `accept-eula` | Accept the Work IQ EULA via the MCP `accept_eula` tool |
| `mcp` | Start MCP proxy server (stdio) — also the implicit mode when no TTY |
| `serve` | Start OpenAI-compatible HTTP API on localhost |
| `json` | Interactive JSON-RPC testing mode |
| `version` | Show workiq CLI version (passthrough) |
| `help` | Show usage summary |

## Configure

### Claude Code

**Project-level** (adds to `.mcp.json` in current directory):

```bash
claude mcp add --transport stdio workiq --scope project -- npx -y @stuffbucket/workiq-proxy
```

**User-level** (available in all projects):

```bash
claude mcp add --transport stdio workiq --scope user -- npx -y @stuffbucket/workiq-proxy
```

Or add to `.mcp.json` manually:

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

### VS Code / GitHub Copilot

**Project-level** — add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@stuffbucket/workiq-proxy"]
    }
  }
}
```

**User-level** — add to VS Code user settings (`settings.json`):

```json
{
  "mcp": {
    "servers": {
      "workiq": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@stuffbucket/workiq-proxy"]
      }
    }
  }
}
```

### OpenAI Codex CLI

Add to `.codex/mcp.json`:

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

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--workiq-cmd` | `npx -y @microsoft/workiq` | Base command to reach the Work IQ CLI (proxy appends `mcp` for MCP mode) |
| `--log-file` | `~/.work-iq-cli/mcp-traffic.log` | Override log file path |
| `--no-log` | `false` | Disable traffic logging |
| `--port` | `11435` | Port for `serve` mode (seeks next available if busy) |

Example with a globally installed Work IQ CLI:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@stuffbucket/workiq-proxy", "--workiq-cmd", "workiq"]
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

The proxy enriches five categories of upstream errors with actionable hints:

| Error pattern | Likely cause | Fix |
|---------------|-------------|-----|
| EULA / license agreement | Work IQ EULA not accepted | Run `workiq-proxy accept-eula` |
| Error 530084 / token protection | Org requires token protection CA policy | Ask IT admin to exempt the Work IQ CLI app |
| AADSTS / security policy | Entra ID conditional access policy blocking | Check the error code and contact IT admin |
| InteractionRequired | Browser login needed | Run `workiq-proxy ask -q "What's on my calendar?"` to re-authenticate |
| Failed to create conversation | Auth token expired or Copilot license inactive | Run `workiq-proxy ask -q "What's on my calendar?"` to re-authenticate |

**Server not connecting** — Check `~/.work-iq-cli/mcp-traffic.log` for the full JSON-RPC transcript.

## Development

```bash
# Build
make build

# Test
make test

# Lint (Go + JS)
make lint

# Cross-compile all platforms
make all
```

## License

MIT — see [LICENSE](LICENSE). Charmbracelet libraries are used under the MIT license (see third-party notices in LICENSE).
