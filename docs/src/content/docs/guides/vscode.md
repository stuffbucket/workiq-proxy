---
draft: false
title: VS Code / GitHub Copilot
description: Configure workiq-proxy as an MCP server in VS Code.
sidebar:
  order: 2
---

Connect workiq-proxy to VS Code so GitHub Copilot can search your M365 emails, docs, chats, and meetings while you code.

> **Shortcut:** Run `workiq-proxy install copilot` from any terminal — it creates the configuration for you automatically.

## Project-level

Use this to share the config with your team via git. Add to `.vscode/mcp.json` in your project:

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

Use this to enable workiq-proxy in every workspace. Add to your VS Code user settings (`Cmd+Shift+P` → "Preferences: Open User Settings (JSON)"):

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

Open the Output panel (`Cmd+Shift+U`), select **GitHub Copilot Chat** from the dropdown, and look for a line showing the `workiq` server connected with 9 tools.

Alternatively, type `@` in the Copilot chat and look for the Work IQ tools in the tool list.

## Usage

Once connected, Copilot can call the Work IQ tools directly. Try asking:

- *"Search my emails from last week about the budget review"*
- *"Find documents in SharePoint about project onboarding"*
- *"What meetings do I have tomorrow?"*

Copilot picks the right tool automatically based on your question.
