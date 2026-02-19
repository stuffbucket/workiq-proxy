# Microsoft Work IQ — MCP Surface Area Reference

Last probed: 2026-02-19 (package version 0.2.8, server version 1.0.0)

## Server Identity

| Field | Value |
|-------|-------|
| Name | WorkIQ |
| Server version | 1.0.0 |
| Package | `@microsoft/workiq` (npm) |
| Package version | 0.2.8 |
| MCP protocol | 2024-11-05 |
| Transport | stdio (newline-delimited JSON-RPC 2.0) |
| Binary | Proprietary .NET/CoreCLR native binary (~119 MB unpacked) |

## Declared Capabilities

```json
{
  "logging": {},
  "tools": { "listChanged": true }
}
```

**Supported:** `tools`, `logging`
**Not supported:** `prompts` (-32601), `resources` (-32601)

The `tools.listChanged: true` flag means the tool list may change dynamically (e.g., after EULA acceptance).

## Tools (2 total)

### `accept_eula`

Accepts the End User License Agreement. Must be called before `ask_work_iq` will function.

```json
{
  "name": "accept_eula",
  "description": "Accept the End User License Agreement (EULA) to enable full access to WorkIQ tools. The EULA URL should be https://github.com/microsoft/work-iq-mcp. This action requires explicit user confirmation and cannot be automatically invoked.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "eulaUrl": {
        "description": "The URL of the EULA, should be https://github.com/microsoft/work-iq-mcp",
        "type": "string"
      }
    },
    "required": ["eulaUrl"]
  },
  "annotations": {
    "destructiveHint": true
  }
}
```

**Parameters:**
- `eulaUrl` (string, required) — Must be `https://github.com/microsoft/work-iq-mcp`

**Annotations:**
- `destructiveHint: true` — MCP clients should prompt user for confirmation before executing

**EULA acceptance flow — see [EULA Acceptance](#eula-acceptance) below.**

### `ask_work_iq`

Sends a natural language question to the Microsoft 365 Copilot Chat API backend.

```json
{
  "name": "ask_work_iq",
  "description": "Ask a question to Microsoft 365 Copilot for information about emails, meetings, files, and other M365 data.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "question": {
        "description": "The question to ask",
        "type": "string"
      }
    },
    "required": ["question"]
  }
}
```

**Parameters:**
- `question` (string, required) — Free-form natural language query

**No annotations** — clients may call without extra confirmation prompts.

**Queryable data types** (all via this single tool):
- Emails
- Calendar/meetings
- Documents (SharePoint, OneDrive)
- Teams messages (chats and channels)
- People/org information
- Meeting transcripts

## Unsupported MCP Methods

| Method | Error code | Message |
|--------|-----------|---------|
| `resources/list` | -32601 | "Method 'resources/list' is not available." |
| `prompts/list` | -32601 | "Method 'prompts/list' is not available." |

This is the root cause of the Claude Code compatibility issue ([GitHub #38](https://github.com/microsoft/work-iq-mcp/issues/38)) — if a client calls these methods without checking capabilities first, the -32601 error may break the connection.

---

## EULA Acceptance

The EULA gates access to `ask_work_iq`. There are three ways to accept it:

### Option 1: CLI (pre-accept before using MCP)

```bash
# If installed globally:
workiq accept-eula

# If using npx:
npx -y @microsoft/workiq accept-eula
```

This is the **recommended approach** for Claude Code users. Accept the EULA once via CLI, then the MCP server will not require it again. State is stored in `~/.work-iq-cli/` and shared between CLI and MCP modes.

### Option 2: MCP tool call (from within an AI assistant)

The AI assistant calls the `accept_eula` tool with `eulaUrl` set to `https://github.com/microsoft/work-iq-mcp`. Because `destructiveHint: true` is set, the MCP client will prompt the user for confirmation before executing.

How this works in different clients:

| Client | Behavior |
|--------|----------|
| **Claude Code (CLI)** | Prompts user in terminal to approve the tool call |
| **Claude Code (VS Code)** | Prompts user to approve the tool call |
| **VS Code (GitHub Copilot)** | Shows confirmation dialog |
| **GitHub Copilot CLI** | Inline first-use consent prompt |

### Option 3: Interactive CLI session

```bash
workiq ask
# The first interactive session triggers EULA acceptance
```

### State persistence

- Stored in: `~/.work-iq-cli/` (macOS/Linux) or `%USERPROFILE%\.work-iq-cli\` (Windows)
- Auth tokens: `msal_token_cache.dat` in the same directory
- EULA acceptance persists across CLI and MCP modes
- Deleting `~/.work-iq-cli/` resets all state (EULA + auth)

---

## Authentication

After EULA acceptance, the first data query triggers a browser-based Microsoft Entra ID OAuth flow.

### Required delegated permissions (admin-consented)

| Permission | Description |
|-----------|-------------|
| `Sites.Read.All` | Read items in all site collections |
| `Mail.Read` | Read user mail |
| `People.Read.All` | Read all users' relevant people lists |
| `OnlineMeetingTranscript.Read.All` | Read all transcripts of online meetings |
| `Chat.Read` | Read user chat messages |
| `ChannelMessage.Read.All` | Read all channel messages |
| `ExternalItem.Read.All` | Read external items |

### Admin consent URL

```
https://login.microsoftonline.com/{tenant-id}/adminconsent?client_id=ba081686-5d24-4bc6-a0d6-d034ecffed87
```

### Authentication limitations

- No device-code flow (no headless/SSH auth)
- `BROWSER` env var not respected ([#43](https://github.com/microsoft/work-iq-mcp/issues/43))
- Tokens can expire during long sessions
- Throttling after ~30 consecutive calls ([#40](https://github.com/microsoft/work-iq-mcp/issues/40))

---

## Claude Code Configuration

### Add via CLI

```bash
# Project-level (shared via git):
claude mcp add --transport stdio workiq --scope project -- npx -y @microsoft/workiq mcp

# With specific tenant:
claude mcp add --transport stdio workiq --scope project -- npx -y @microsoft/workiq mcp --tenant-id YOUR-TENANT-GUID

# User-level (personal, all projects):
claude mcp add --transport stdio workiq --scope user -- npx -y @microsoft/workiq mcp
```

### Manual `.mcp.json` (project root)

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@microsoft/workiq", "mcp"]
    }
  }
}
```

With tenant ID:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@microsoft/workiq", "mcp", "--tenant-id", "YOUR-TENANT-GUID"]
    }
  }
}
```

### Recommended setup sequence

1. Install: `npm install -g @microsoft/workiq` (or use npx)
2. Accept EULA: `workiq accept-eula`
3. Test auth: `workiq ask -q "What's on my calendar?"`
4. Configure Claude Code: `claude mcp add --transport stdio workiq --scope project -- npx -y @microsoft/workiq mcp`
5. Verify: run `/mcp` inside Claude Code to check server status

---

## Known Issues

| Issue | Status | Description |
|-------|--------|-------------|
| [#38](https://github.com/microsoft/work-iq-mcp/issues/38) | Open | Claude Code -32601 error on tool discovery |
| [#40](https://github.com/microsoft/work-iq-mcp/issues/40) | Open | Throttling after ~30 calls per session |
| [#43](https://github.com/microsoft/work-iq-mcp/issues/43) | Open | BROWSER env var not respected |
| [#46](https://github.com/microsoft/work-iq-mcp/issues/46) | Open | No device-code flow for headless environments |
| [#49](https://github.com/microsoft/work-iq-mcp/issues/49) | Open | "Failed to create conversation" with no diagnostics |

---

## Probing Tool

The tool definitions in this document were extracted using `tools/mcp-probe.js`. To re-probe:

```bash
node tools/mcp-probe.js npx -y @microsoft/workiq mcp
```

Raw probe output is saved in `docs/mcp-surface-area.json`.
