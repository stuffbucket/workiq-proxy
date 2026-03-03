---
# AUTO-GENERATED — do not edit by hand.
# Run "make docs" or "go generate ./cmd/workiq-proxy/" to regenerate.
title: CLI Reference
description: All commands, flags, and REPL slash commands for workiq-proxy.
sidebar:
  order: 10
---

## Subcommands

| Command | Description |
|---------|-------------|
| `install` | Register with an MCP client (claude, copilot) |
| `accept-eula` | Accept the Work IQ EULA |
| `ask -q "..."` | Ask Microsoft 365 Copilot a question |
| `mcp` | Start MCP proxy server (stdio) |
| `serve` | Start OpenAI-compatible HTTP API (localhost) |
| `web` | Open browser-based terminal UI |
| `json` | Interactive JSON-RPC testing mode |
| `version` | Show component versions |
| `licenses` | Print license and third-party disclosures |
| `help` | Show help |

## Quick Start

These are the commands most people need:

| Command | Description |
|---------|-------------|
| `workiq-proxy` | Start an interactive chat session |
| `workiq-proxy install claude` | Register with Claude Code |
| `workiq-proxy install copilot` | Register with GitHub Copilot |
| `workiq-proxy accept-eula` | Accept the Work IQ EULA |
| `workiq-proxy ask -q "..."` | Ask a question directly |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--log-file` | *(none)* | Override log file path (default: $XDG_STATE_HOME/workiq-proxy/workiq-proxy.log) |
| `--no-log` | `false` | Disable traffic logging |
| `--port` | `11435` | Port for serve mode (seeks next available if busy) |
| `--workiq-cmd` | `npx -y @microsoft/workiq` | Base command to reach the Work IQ CLI |

## REPL Slash Commands

When running in interactive mode (no arguments), these slash commands are available:

| Command | Description |
|---------|-------------|
| `/emails [opts]` | Search emails (from: subject: date:) |
| `/docs [opts]` | Search documents (name: site: type:) |
| `/chats [opts]` | Search chats (with: date:) |
| `/channels [opts]` | Search channels (team: channel: date:) |
| `/meetings [opts]` | Search meetings (by: subject: date:) |
| `/people [opts]` | Search people (name: dept: project:) |
| `/accept-eula` | Accept the Work IQ EULA |
| `/title <name>` | Set terminal/tab title |
| `/tools` | List available MCP tools |
| `/help` | Show available commands |
| `/quit` | Exit |

## Modes of Operation

| Condition | Behavior |
|-----------|----------|
| No arguments, interactive terminal | Launches the interactive REPL |
| No arguments, piped stdin (no TTY) | MCP proxy mode (how MCP clients invoke it) |
| `mcp` subcommand | MCP proxy mode (explicit) |
| `serve` subcommand | OpenAI-compatible HTTP API on localhost |
| `web` subcommand | Browser-based terminal UI |
| `json` subcommand | Interactive JSON-RPC testing |
| Any other arguments | Passed through to the Work IQ CLI |

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
