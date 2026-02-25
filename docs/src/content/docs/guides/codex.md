---
draft: false
title: Codex CLI
description: Configure workiq-proxy in OpenAI Codex CLI.
sidebar:
  order: 3
---

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

## Usage

Once configured, Codex can access your M365 data through the synthetic search tools. The tools are structured to give the AI model clear parameters, so queries like *"find my emails from Sarah about the Q3 report"* map directly to `search_emails` with `from` and `subject` parameters.
