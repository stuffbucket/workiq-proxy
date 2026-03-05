# workiq-proxy

Extends [Microsoft Work IQ](https://github.com/microsoft/work-iq-mcp) with broad MCP client support, structured search tools, an interactive REPL, and an OpenAI-compatible HTTP API.

## What it does

workiq-proxy enhances Work IQ's MCP server so it works seamlessly with Claude Code, VS Code, Codex, and other MCP clients.

In MCP mode the proxy sits between the MCP client and the Work IQ server:

```
MCP Client <-stdio-> workiq-proxy <-stdio-> workiq mcp
```

It:

1. **Extends MCP capabilities** — Adds support for `prompts/list`, `resources/list`, and `resources/templates/list` so every MCP client connects cleanly
2. **Advertises full capabilities** — Ensures clients see a fully capable server during initialization
3. **Future-proof** — Automatically steps aside when Work IQ gains these capabilities natively
4. **Error enrichment** — Wraps opaque errors with troubleshooting context (EULA not accepted, token protection / error 530084, Entra ID conditional access / AADSTS, interactive login required, and "Failed to create conversation")
5. **Synthetic tools** — Exposes 7 domain-specific tools (`search_emails`, `search_documents`, `search_chats`, `search_channels`, `search_meetings`, `search_people`, `search_external`) backed by `ask_work_iq`
6. **Interactive REPL** — When run from a terminal with no arguments, launches a Bubble Tea UI with slash commands (`/ask`, `/emails`, `/docs`, `/chats`, `/channels`, `/meetings`, `/people`, `/accept-eula`, `/tools`, `/help`, `/quit`), session history, and glamour markdown rendering
7. **OpenAI-compatible HTTP API** — `workiq-proxy serve` starts a local server with `/v1/chat/completions` (streaming and non-streaming) and `/v1/models`
8. **CLI passthrough** — When run from an interactive terminal with arguments, delegates to the underlying `workiq` CLI (EULA acceptance, auth, queries) so you only need one package
9. **Traffic logging** — Logs all JSON-RPC messages to `~/.work-iq-cli/mcp-traffic.log`

## Install

```bash
curl -fsSL https://stuffbucket.github.io/workiq-proxy/install.sh | sh
```

The installer detects (or installs) Node.js and sets up workiq-proxy via npm.

On Windows (PowerShell):

```powershell
irm https://stuffbucket.github.io/workiq-proxy/install.ps1 | iex
```

Alternatively, install directly with npm:

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

## Programmatic API

Use workiq-proxy as a dependency to embed Work IQ in your own Node.js project:

```bash
npm install @stuffbucket/workiq-proxy
```

### Pipe-based (no port, no network)

```javascript
const { createClient } = require("@stuffbucket/workiq-proxy");

const client = await createClient();
const answer = await client.ask("What's on my calendar?");
console.log(answer);

const emails = await client.searchEmails({ from: "alice", date_range: "last week" });
const docs   = await client.searchDocuments({ keywords: "quarterly report" });
const chats  = await client.searchChats({ person: "bob", keywords: "standup" });

await client.close();
```

`createClient()` spawns workiq-proxy over stdio pipes — no port, no HTTP. The MCP handshake happens automatically.

| Method | Description |
|--------|-------------|
| `ask(question)` | Ask Microsoft 365 Copilot a question |
| `searchEmails(params)` | Search emails (`from`, `subject`, `keywords`, `date_range`) |
| `searchDocuments(params)` | Search documents (`filename`, `keywords`, `site`, `file_type`) |
| `searchChats(params)` | Search chats (`person`, `keywords`, `date_range`, `channel`) |
| `callTool(name, args)` | Call any MCP tool by name |
| `listTools()` | List all available tools |
| `close()` | Shut down the child process |

Options: `createClient({ workiqCmd?, noLog? })`

### HTTP server (OpenAI-compatible)

If you need an HTTP API (e.g. for non-Node consumers):

```javascript
const { createServer } = require("@stuffbucket/workiq-proxy");

const server = await createServer();           // starts serve --json
const models = await server.listModels();      // /v1/models
const reply  = await server.chat("What's on my calendar?");
console.log(reply.choices[0].message.content);
await server.close();
```

| Method | Description |
|--------|-------------|
| `server.url` | Base URL (e.g. `http://127.0.0.1:11435`) |
| `server.listModels()` | List available models |
| `server.chat(message)` | Send a chat message (string or full OpenAI request body) |
| `server.chat(body, { stream: true })` | Stream SSE chunks |
| `server.onEvent(fn)` | Subscribe to JSONL log events; returns unsubscribe function |
| `server.events` | Array of all captured log events |
| `server.close()` | Shut down the server |

Options: `createServer({ port?, workiqCmd?, noLog? })`

## License

MIT — see [LICENSE](LICENSE). Charmbracelet libraries are used under the MIT license (see third-party notices in LICENSE).
