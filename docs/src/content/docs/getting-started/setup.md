---
draft: false
title: Setup
description: Accept the EULA, authenticate, and verify the proxy works.
sidebar:
  order: 2
---

After installing, you have three one-time steps before your AI assistant can search your M365 data. This takes about two minutes.

## 1. Accept the EULA

Work IQ requires you to accept its End User License Agreement before it will respond to queries.

```bash
npx -y @stuffbucket/workiq-proxy ask
```

This starts an interactive session that walks you through EULA acceptance. You only need to do this once — the acceptance persists in `~/.work-iq-cli/`.

## 2. Authenticate

The first time you run a data query, your browser will open to a Microsoft sign-in page. Sign in with your work account and grant the requested permissions.

> A tenant admin must grant consent before users can authenticate — see [Admin Consent](/workiq-proxy/reference/admin/) for the required permissions and consent URL.

## 3. Verify

Confirm everything works:

```bash
npx -y @stuffbucket/workiq-proxy ask -q "What's on my calendar?"
```

If you see calendar results, you're ready to connect an AI assistant. Head to the guide for your tool:

- [Claude Code](/workiq-proxy/guides/claude-code/)
- [VS Code / GitHub Copilot](/workiq-proxy/guides/vscode/)
- [Codex CLI](/workiq-proxy/guides/codex/)
- [OpenAI-compatible API](/workiq-proxy/guides/serve-mode/)

All local state (EULA acceptance, tokens, logs) is stored in `~/.work-iq-cli/` — see [State Files](/workiq-proxy/reference/state-files/) for details.
