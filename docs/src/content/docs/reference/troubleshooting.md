---
draft: false
title: Troubleshooting
description: Common issues and how to resolve them.
sidebar:
  order: 3
---

## "Failed to create conversation"

**Cause:** Your auth token expired or your Microsoft 365 Copilot license is not active.

**Solution:** Re-authenticate from the terminal:

```bash
npx -y @stuffbucket/workiq-proxy ask -q "What's on my calendar?"
```

This triggers a browser auth flow and refreshes your tokens.

## Server not connecting

**Check the traffic log:**

```bash
cat ~/.work-iq-cli/mcp-traffic.log
```

The log contains the full JSON-RPC transcript between the client and the proxy. Look for error responses or connection failures.

## `-32601` errors in client

If you see "Method not found" errors, the client may be connecting directly to the Work IQ server instead of through the proxy. Verify your MCP configuration points to `@stuffbucket/workiq-proxy`, not `@microsoft/workiq`.

## No tools visible

After connecting, you should see 9 tools. If you see 0 or only 2:

1. **EULA not accepted** — Run `npx -y @stuffbucket/workiq-proxy ask` to accept it
2. **Proxy not in the path** — Confirm `npx -y @stuffbucket/workiq-proxy` resolves correctly

## Browser auth doesn't open

Work IQ uses browser-based authentication rather than the `BROWSER` environment variable or device-code flow. You need:

- A desktop environment with a browser
- Direct terminal access (not SSH without X11 forwarding)

## Throttling

After ~30 consecutive calls to `ask_work_iq`, the M365 backend may throttle requests. Wait a few minutes and retry. See [work-iq-mcp#40](https://github.com/microsoft/work-iq-mcp/issues/40) for details.

## Reset everything

Delete the state directory to start fresh:

```bash
rm -rf ~/.work-iq-cli
```

You'll need to re-accept the EULA and re-authenticate.
