---
draft: false
title: Install
description: Install workiq-proxy via npm or download a binary.
sidebar:
  order: 1
---

workiq-proxy lets your AI assistant search your Microsoft 365 emails, documents, chats, meetings, and people. Pick any installation method below — you'll be up and running in under a minute.

## Before you start

workiq-proxy extends [Microsoft Work IQ](https://github.com/microsoft/work-iq-mcp). You'll need:

- **Node.js 18+** — the underlying Work IQ CLI runs on Node
- **Microsoft 365 Copilot license** — your tenant must have M365 Copilot enabled
- **Admin-consented permissions** — a tenant admin must grant delegated permissions once (see [Admin Consent](/workiq-proxy/reference/admin/))

## npm (recommended)

```bash
npm install -g @stuffbucket/workiq-proxy
```

This installs the `workiq-proxy` command globally. The npm package downloads the correct platform binary automatically.

## npx (no install)

Every configuration example in this documentation uses `npx`, so you never need a global install:

```bash
npx -y @stuffbucket/workiq-proxy
```

## Binary download

Download a prebuilt binary from [GitHub Releases](https://github.com/stuffbucket/workiq/releases) and place it somewhere in your `PATH`.

| Platform | Binary |
|----------|--------|
| macOS (Apple Silicon) | `workiq-proxy-darwin-arm64` |
| macOS (Intel) | `workiq-proxy-darwin-x64` |
| Linux (x64) | `workiq-proxy-linux-x64` |
| Linux (ARM64) | `workiq-proxy-linux-arm64` |
| Windows (x64) | `workiq-proxy-win-x64.exe` |
| Windows (ARM64) | `workiq-proxy-win-arm64.exe` |
