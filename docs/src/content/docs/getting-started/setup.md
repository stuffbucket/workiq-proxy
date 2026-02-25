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

A tenant admin must grant consent before users can authenticate — see [Admin Consent](/workiq-proxy/reference/admin/) for the required permissions and consent URL.

## 3. Verify

Confirm auth works non-interactively:

```bash
npx -y @stuffbucket/workiq-proxy ask -q "What's on my calendar?"
```

If you see results, you're ready to connect an MCP client.

All local state (EULA, tokens, logs) is stored in `~/.work-iq-cli/` — see [State Files](/workiq-proxy/reference/state-files/) for details.
