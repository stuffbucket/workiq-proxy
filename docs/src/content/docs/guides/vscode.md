---
draft: false
title: VS Code / GitHub Copilot
description: Configure workiq-proxy as an MCP server in VS Code.
sidebar:
  order: 2
---

## Project-level

Add to `.vscode/mcp.json` in your project:

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

## User-level

Add to your VS Code user settings (`settings.json`):

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

## Verify

Check the MCP server status in the VS Code output panel. The server should connect and expose 9 tools.

## Usage

Once connected, GitHub Copilot can call the Work IQ tools directly. Ask questions like:

- *"Search my emails from last week about the budget review"*
- *"Find documents in SharePoint about project onboarding"*
- *"What meetings do I have tomorrow?"*

Copilot picks the right tool automatically based on your question.
