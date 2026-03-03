---
draft: false
title: Codex CLI
description: Configure workiq-proxy in OpenAI Codex CLI.
sidebar:
  order: 3
---

Connect workiq-proxy to OpenAI's Codex CLI so it can search your M365 data during coding sessions.

## Configuration

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

## Verify

Start Codex and ask it something that requires M365 data:

```
Find my emails from Sarah about the Q3 report
```

Codex should call `search_emails` with the appropriate parameters. If you see results, the connection is working.

## How it works

The proxy gives Codex 7 structured search tools (emails, documents, chats, channels, meetings, people, and external items). Each tool has explicit parameters like `from`, `subject`, and `date_range`, so Codex can map your request directly to the right query without composing free-form text.
