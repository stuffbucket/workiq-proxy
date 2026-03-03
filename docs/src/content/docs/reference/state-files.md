---
draft: false
title: State Files
description: Local state directory and file reference.
sidebar:
  order: 5
---

All local state lives in `~/.work-iq-cli/`:

| File | Purpose |
|------|---------|
| EULA acceptance flag | Unlocks `ask_work_iq` tool |
| `msal_token_cache.dat` | Cached Microsoft Entra ID auth tokens |
| `mcp-traffic.log` | JSON-RPC message log (when logging is enabled) |

Deleting `~/.work-iq-cli/` resets everything — EULA acceptance, cached auth tokens, and logs. You will need to re-accept the EULA and re-authenticate.
