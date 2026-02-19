# Work IQ MCP Proxy — Implementation Plan

## Goal

Build a Go-based MCP compatibility proxy that wraps the proprietary `workiq mcp` binary, fixing Claude Code (and other MCP client) compatibility issues. Distribute via npm (`@stuffbucket/workiq-proxy`) to match the workiq installation pattern across macOS, Linux, and Windows.

## Background

- Work IQ's MCP server only declares `tools` and `logging` capabilities
- MCP clients like Claude Code call `prompts/list` and `resources/list`, which return -32601 and can break the connection ([GitHub #38](https://github.com/microsoft/work-iq-mcp/issues/38))
- The workiq binary is proprietary .NET/CoreCLR — we can't fix it, only wrap it
- The npm package `@microsoft/workiq` is just a delivery mechanism for native binaries — no JavaScript actually runs

## Decisions made

- **Language:** Go — single static binary, no runtime dependency, cross-compiles trivially
- **Package name:** `@stuffbucket/workiq-proxy`
- **npm scope:** `@stuffbucket` (org owned by `stuffbucketco`)
- **GitHub repo:** `stuffbucket/workiq-proxy` (public, required for postinstall binary downloads from GitHub Releases)
- **Distribution:** npm package with a `postinstall.js` that downloads the correct platform binary from GitHub Releases (not bundled — keeps the npm package tiny)
- **Binary naming:** `workiq-proxy` (not `workiq-shim`)

## What the proxy does

1. **Intercepts unsupported methods** — Returns empty valid responses for `prompts/list`, `resources/list`, `resources/templates/list` instead of forwarding to workiq (which returns -32601)
2. **Patches initialize capabilities** — Adds `prompts` and `resources` to the capability response so clients see a fully capable server
3. **Self-disabling** — Reads the real server's initialize response; if a future workiq version declares prompts or resources, stops intercepting those methods
4. **Auth pre-flight check** — Before forwarding `ask_work_iq`, checks for `~/.work-iq-cli/msal_token_cache.dat`; returns an actionable error if missing instead of letting the binary block stdio with a browser flow
5. **Error enrichment** — Wraps opaque errors like "Failed to create conversation" with actionable context
6. **Traffic logging** — Appends all JSON-RPC messages to `~/.work-iq-cli/mcp-traffic.log` for debugging (workiq has no verbose mode)

## Architecture

```
MCP Client ←stdio→ workiq-proxy (Go binary) ←stdio→ workiq mcp (.NET binary)
```

The proxy is a transparent stdio proxy. It parses newline-delimited JSON-RPC 2.0 messages, applies the above logic, and forwards everything else untouched.

## Implementation steps

### Step 1: Go proxy binary

**File:** `cmd/workiq-proxy/main.go`

Implement in a single file (~200-250 lines):

- `main()` — Parse flags, spawn `workiq mcp` (or `npx -y @microsoft/workiq mcp`) as child process, wire up stdio pipes, handle signals
- Line buffer — Read stdio chunks, split on newlines, parse JSON-RPC messages
- Inbound router — Check `method` field: intercept `prompts/list`, `resources/list`, `resources/templates/list` with empty responses; auth pre-flight on `tools/call` for `ask_work_iq`; forward everything else to child stdin
- Outbound handler — Read child stdout: patch `initialize` response capabilities (self-disabling check here); enrich error responses on `tools/call` results; forward to own stdout
- Pending map — Track `id → method` for outbound responses so we know which responses to patch/enrich
- Logging — Append timestamped JSON to log file; fail silently if logging breaks
- Cleanup — Forward SIGINT/SIGTERM to child, exit when child exits, close stdin on EOF

**Flags:**
- `--workiq-cmd` — Override the command to spawn (default: `npx -y @microsoft/workiq mcp`). Allows users to point at a globally installed binary: `--workiq-cmd "workiq mcp"`
- `--log-file` — Override log file path (default: `~/.work-iq-cli/mcp-traffic.log`)
- `--no-log` — Disable traffic logging

**Go module:** `github.com/stuffbucket/workiq-proxy`

**Dependencies:** None beyond Go stdlib (`os`, `os/exec`, `bufio`, `encoding/json`, `path/filepath`, `log`, `flag`, `runtime`)

### Step 2: Cross-compilation

**File:** `Makefile`

Build for 6 targets:
```
GOOS=darwin  GOARCH=amd64  → workiq-proxy-darwin-x64
GOOS=darwin  GOARCH=arm64  → workiq-proxy-darwin-arm64
GOOS=linux   GOARCH=amd64  → workiq-proxy-linux-x64
GOOS=linux   GOARCH=arm64  → workiq-proxy-linux-arm64
GOOS=windows GOARCH=amd64  → workiq-proxy-win-x64.exe
GOOS=windows GOARCH=arm64  → workiq-proxy-win-arm64.exe
```

### Step 3: npm package wrapper

**File:** `npm/package.json`

```json
{
  "name": "@stuffbucket/workiq-proxy",
  "version": "1.0.0",
  "description": "MCP compatibility proxy for Microsoft Work IQ",
  "bin": {
    "workiq-proxy": "run.js"
  },
  "scripts": {
    "postinstall": "node postinstall.js"
  }
}
```

**File:** `npm/postinstall.js` (~30 lines)

- Detect `process.platform` and `process.arch`
- Map to the correct binary name
- Download the binary from `https://github.com/stuffbucket/workiq-proxy/releases/download/v{version}/workiq-proxy-{platform}-{arch}`
- Write to the package's `bin/` directory
- Make it executable (`chmod +x`)

**File:** `npm/run.js` (~10 lines)

- Fallback runner: resolve the platform-specific binary and `execFileSync` it with passed-through args
- Only used if postinstall download didn't work

### Step 4: GitHub Actions CI

**File:** `.github/workflows/release.yml`

Triggered on version tags (`v*`):

1. Set up Go
2. Run `go build` for all 6 targets
3. Create GitHub Release with the binaries attached
4. Publish npm package (postinstall.js downloads binaries from the release)

Needs `NPM_TOKEN` as a repository secret.

### Step 5: Documentation updates

Update existing docs:

- `docs/workiq-mcp-reference.md` — Add proxy installation instructions, update Claude Code config to use proxy
- `docs/eula-acceptance.md` — Note that the proxy provides an auth pre-flight check

Add new doc:

- `README.md` — How to install, configure, verify, and troubleshoot the proxy

## MCP client configuration (end result)

### Claude Code

```bash
claude mcp add --transport stdio workiq --scope project -- npx -y @stuffbucket/workiq-proxy
```

Or in `.mcp.json`:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@stuffbucket/workiq-proxy"]
    }
  }
}
```

### VS Code

```json
{
  "workiq": {
    "command": "npx",
    "args": ["-y", "@stuffbucket/workiq-proxy"],
    "tools": ["*"]
  }
}
```

### Direct binary (no npm)

Download from GitHub Releases, then:

```json
{
  "mcpServers": {
    "workiq": {
      "type": "stdio",
      "command": "/path/to/workiq-proxy"
    }
  }
}
```

## Testing

1. **Unit test the proxy** — Send JSON-RPC messages via stdin, verify correct responses on stdout. Use the existing `tools/mcp-probe.js` to probe the proxy the same way we probed the raw server.
2. **Integration test** — Run the proxy wrapping the real `workiq mcp` binary. Verify:
   - `initialize` response includes prompts + resources capabilities
   - `prompts/list` returns empty list (no -32601)
   - `resources/list` returns empty list (no -32601)
   - `tools/list` returns the real 2 tools from workiq
   - Auth pre-flight catches missing token cache
   - Traffic log file is written
3. **Claude Code end-to-end** — Add the proxy to `.mcp.json`, start Claude Code, verify `/mcp` shows the server connected and tools available

## Open decisions for the implementing agent

- [ ] Whether to support `--workiq-cmd` pointing at a globally installed binary vs always using npx
- [ ] Log file rotation strategy (or just let it grow)

## Reference files (all under `contrib/`)

- `contrib/tools/workiq-proxy.js` — JavaScript prototype (working, tested against real workiq server). Use as reference for the Go port.
- `contrib/tools/mcp-probe.js` — MCP server probe tool. Use for testing.
- `contrib/docs/mcp-surface-area.json` — Raw MCP tool definitions from the real server.
- `contrib/docs/workiq-mcp-reference.md` — Full surface area documentation.
- `contrib/docs/eula-acceptance.md` — EULA + auth gate documentation.
