---
draft: false
title: Claude Code
description: Configure workiq-proxy as an MCP server in Claude Code.
sidebar:
  order: 1
---

Connect workiq-proxy to Claude Code so Claude can search your M365 emails, docs, chats, and meetings while you work.

## One-line setup

The fastest path. Pick **project-level** to share the config with your team via git, or **user-level** to enable it everywhere on your machine.

### Project-level (shared via git)

```bash
claude mcp add --transport stdio workiq --scope project -- npx -y @stuffbucket/workiq-proxy
```

This writes to `.mcp.json` in your current directory.

### User-level (all projects)

```bash
claude mcp add --transport stdio workiq --scope user -- npx -y @stuffbucket/workiq-proxy
```

> **Shortcut:** `workiq-proxy install claude` does the same thing — it detects Claude Code and registers the server for you.

## Manual configuration

If you prefer to edit the config file directly, add to `.mcp.json`:

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

If you've installed `@microsoft/workiq` globally and want to skip the `npx` download on each launch:

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

Run `/mcp` inside Claude Code. You should see **workiq** listed as a connected server with 9 tools:

- `accept_eula`
- `ask_work_iq`
- `search_emails`
- `search_documents`
- `search_chats`
- `search_channels`
- `search_meetings`
- `search_people`
- `search_external`

Try asking Claude: *"Search my emails from last week about the budget review"* — it will call the right tool automatically.
