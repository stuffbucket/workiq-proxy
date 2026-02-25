---
draft: false
title: Claude Code
description: Configure workiq-proxy as an MCP server in Claude Code.
sidebar:
  order: 1
---

## One-line setup

### Project-level (shared via git)

```bash
claude mcp add --transport stdio workiq --scope project -- npx -y @stuffbucket/workiq-proxy
```

This writes to `.mcp.json` in your current directory.

### User-level (all projects)

```bash
claude mcp add --transport stdio workiq --scope user -- npx -y @stuffbucket/workiq-proxy
```

## Manual configuration

Add to `.mcp.json`:

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

### With a globally installed Work IQ CLI

If you installed `@microsoft/workiq` globally and want to skip `npx`:

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

Run `/mcp` inside Claude Code to check server status. You should see 9 tools:

- `accept_eula`
- `ask_work_iq`
- `search_emails`
- `search_documents`
- `search_chats`
- `search_channels`
- `search_meetings`
- `search_people`
- `search_external`
