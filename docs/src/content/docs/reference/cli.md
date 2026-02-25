---
draft: false
title: CLI Flags
description: Command-line flags and subcommands for workiq-proxy.
sidebar:
  order: 1
---

## Subcommands

| Subcommand | Description |
|-----------|-------------|
| *(none)* | MCP proxy mode (when stdin is not a terminal) |
| `ask` | Interactive CLI — accepts EULA and runs queries |
| `ask -q "..."` | Non-interactive query |
| `mcp` | Explicit MCP proxy mode (useful when forcing MCP from a terminal) |
| `serve` | Start OpenAI-compatible HTTP API |
| `accept-eula` | Accept EULA via MCP tool call |
| `json` | Interactive JSON-RPC mode for debugging |
| `help` | Show usage |

When run from an interactive terminal with no subcommand, workiq-proxy detects the TTY and launches the Work IQ CLI interactively.

When stdin is piped (non-TTY), it defaults to MCP proxy mode — this is how MCP clients like Claude Code invoke it.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--workiq-cmd` | `npx -y @microsoft/workiq` | Base command to reach the Work IQ CLI. The proxy appends `mcp` for MCP mode. |
| `--log-file` | `~/.work-iq-cli/mcp-traffic.log` | Override log file path |
| `--no-log` | `false` | Disable traffic logging |
| `--port` | `11435` | Port for serve mode (seeks next available if busy) |

## Examples

### Use a globally installed Work IQ binary

```bash
workiq-proxy --workiq-cmd workiq
```

### Disable logging

```bash
workiq-proxy --no-log
```

### Custom log location

```bash
workiq-proxy --log-file /tmp/workiq-debug.log
```

### Serve on a specific port

```bash
workiq-proxy serve --port 9000
```
