---
draft: false
title: Setup
description: Accept the EULA, authenticate, and verify the proxy works.
sidebar:
  order: 2
---

After installing, complete three one-time steps: EULA acceptance, authentication, and verification.

## 1. Accept the EULA

The Work IQ MCP server requires EULA acceptance before it responds to queries.

```bash
npx -y @stuffbucket/workiq-proxy ask
```

The interactive session walks you through EULA acceptance. This only needs to happen once — the state persists in `~/.work-iq-cli/`.

## 2. Authenticate

The first data query triggers a browser-based Microsoft Entra ID OAuth flow. Your browser opens automatically.

### Required delegated permissions

A tenant admin must consent to these permissions for the Work IQ app:

| Permission | What it reads |
|-----------|---------------|
| `Mail.Read` | Your email |
| `Chat.Read` | Teams chat messages |
| `ChannelMessage.Read.All` | Teams channel messages |
| `Sites.Read.All` | SharePoint / OneDrive documents |
| `People.Read.All` | Org directory |
| `OnlineMeetingTranscript.Read.All` | Meeting transcripts |
| `ExternalItem.Read.All` | External connector items |

### Admin consent URL

```
https://login.microsoftonline.com/{tenant-id}/adminconsent?client_id=ba081686-5d24-4bc6-a0d6-d034ecffed87
```

Replace `{tenant-id}` with your Microsoft Entra tenant ID.

## 3. Verify

Confirm auth works non-interactively:

```bash
npx -y @stuffbucket/workiq-proxy ask -q "What's on my calendar?"
```

If you see results, you're ready to connect an MCP client.

## State files

All state lives in `~/.work-iq-cli/`:

| File | Purpose |
|------|---------|
| EULA acceptance flag | Unlocks `ask_work_iq` tool |
| `msal_token_cache.dat` | Cached auth tokens |
| `mcp-traffic.log` | JSON-RPC message log (when logging enabled) |

Deleting `~/.work-iq-cli/` resets everything (EULA + auth).
