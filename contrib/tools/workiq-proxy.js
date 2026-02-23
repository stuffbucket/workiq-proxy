#!/usr/bin/env node
//
// workiq-proxy.js — Compatibility proxy for Work IQ MCP server.
//
// Wraps `workiq mcp` and fixes Claude Code compatibility issues:
//   - Intercepts unsupported methods (prompts/list, resources/list) with empty responses
//   - Patches initialize capabilities to match intercepted methods
//   - Pre-flight auth check before forwarding ask_work_iq calls
//   - Enriches opaque error messages with actionable context
//   - Logs all MCP traffic for debugging
//
// Self-disabling: if a future Work IQ version declares prompts or resources
// capabilities, the proxy stops intercepting those methods and passes through.
//
// Usage (Claude Code .mcp.json):
//   {
//     "mcpServers": {
//       "workiq": {
//         "type": "stdio",
//         "command": "node",
//         "args": ["tools/workiq-proxy.js"]
//       }
//     }
//   }
//

const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");
const os = require("os");

const WORKIQ_STATE_DIR = path.join(os.homedir(), ".work-iq-cli");
const TOKEN_CACHE = path.join(WORKIQ_STATE_DIR, "msal_token_cache.dat");
const LOG_FILE = path.join(WORKIQ_STATE_DIR, "mcp-traffic.log");

// Capabilities the real server is missing — we patch these unless it declares them itself.
let interceptPrompts = true;
let interceptResources = true;

// --- Logging ---

function log(direction, msg) {
  const ts = new Date().toISOString();
  const line = `${ts} ${direction} ${JSON.stringify(msg)}\n`;
  try {
    fs.mkdirSync(WORKIQ_STATE_DIR, { recursive: true });
    fs.appendFileSync(LOG_FILE, line);
  } catch {
    // Logging should never break the proxy.
  }
}

// --- JSON-RPC helpers ---

function rpcResult(id, result) {
  return JSON.stringify({ jsonrpc: "2.0", id, result }) + "\n";
}

// rpcError is reserved for future use when the proxy needs to synthesize
// JSON-RPC error responses (e.g. for auth or method interception).
// function rpcError(id, code, message) {
//   return JSON.stringify({ jsonrpc: "2.0", id, error: { code, message } }) + "\n";
// }

// --- Auth pre-flight ---

function hasAuthToken() {
  try {
    return fs.statSync(TOKEN_CACHE).size > 0;
  } catch {
    return false;
  }
}

// --- Error enrichment ---

function enrichError(result) {
  if (!result || !result.content) return result;
  const text = JSON.stringify(result.content);
  if (text.includes("Failed to create conversation")) {
    result.content.push({
      type: "text",
      text: "\n\n[workiq-proxy] This usually means your auth token expired or your Copilot license is not active. Try running: workiq ask -q \"What's on my calendar?\" in your terminal to re-authenticate.",
    });
  }
  return result;
}

// --- Capability patching ---

function patchCapabilities(response) {
  if (!response.result || !response.result.capabilities) return response;
  const caps = response.result.capabilities;

  // Self-disable: if the server already declares these, don't intercept.
  if (caps.prompts) interceptPrompts = false;
  if (caps.resources) interceptResources = false;

  // Patch in what's missing so Claude Code sees a fully capable server.
  if (interceptPrompts) caps.prompts = {};
  if (interceptResources) caps.resources = {};

  return response;
}

// --- Line buffer ---
// Stdio data events don't align to message boundaries. Buffer partial lines.

function createLineBuffer(onLine) {
  let buf = "";
  return (chunk) => {
    buf += chunk;
    let nl;
    while ((nl = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, nl).trim();
      buf = buf.slice(nl + 1);
      if (line) onLine(line);
    }
  };
}

// --- Start the real server ---

const child = spawn("npx", ["-y", "@microsoft/workiq", "mcp"], {
  stdio: ["pipe", "pipe", "pipe"],
  env: { ...process.env },
});

// Forward child stderr to our stderr (diagnostics passthrough).
child.stderr.on("data", (d) => process.stderr.write(d));

// Pending requests: track id → method so we can enrich responses.
const pending = new Map();

// --- Inbound: Claude Code → proxy ---

process.stdin.on("data", createLineBuffer((line) => {
  let msg;
  try {
    msg = JSON.parse(line);
  } catch {
    // Unparseable — forward as-is.
    child.stdin.write(line + "\n");
    return;
  }

  log(">>>", msg);

  // Intercept unsupported methods.
  if (msg.method === "prompts/list" && interceptPrompts) {
    const resp = rpcResult(msg.id, { prompts: [] });
    log("<<<", { id: msg.id, intercepted: "prompts/list" });
    process.stdout.write(resp);
    return;
  }
  if (msg.method === "resources/list" && interceptResources) {
    const resp = rpcResult(msg.id, { resources: [] });
    log("<<<", { id: msg.id, intercepted: "resources/list" });
    process.stdout.write(resp);
    return;
  }
  if (msg.method === "resources/templates/list" && interceptResources) {
    const resp = rpcResult(msg.id, { resourceTemplates: [] });
    log("<<<", { id: msg.id, intercepted: "resources/templates/list" });
    process.stdout.write(resp);
    return;
  }

  // Auth pre-flight for ask_work_iq.
  if (msg.method === "tools/call" && msg.params?.name === "ask_work_iq") {
    if (!hasAuthToken()) {
      const resp = rpcResult(msg.id, {
        content: [{
          type: "text",
          text: "No cached auth token found. Run this in your terminal first:\n\n  workiq ask -q \"What's on my calendar?\"\n\nThis triggers browser-based authentication and caches the token for MCP use.",
        }],
        isError: true,
      });
      log("<<<", { id: msg.id, intercepted: "auth-precheck-failed" });
      process.stdout.write(resp);
      return;
    }
  }

  // Track request for response enrichment.
  if (msg.id !== undefined && msg.method) {
    pending.set(msg.id, msg.method);
  }

  child.stdin.write(line + "\n");
}));

// --- Outbound: workiq → proxy → Claude Code ---

child.stdout.on("data", createLineBuffer((line) => {
  let msg;
  try {
    msg = JSON.parse(line);
  } catch {
    process.stdout.write(line + "\n");
    return;
  }

  // Patch initialize response capabilities.
  if (msg.id !== undefined && pending.get(msg.id) === "initialize") {
    msg = patchCapabilities(msg);
  }

  // Enrich tool call error responses.
  if (msg.id !== undefined && pending.get(msg.id) === "tools/call" && msg.result) {
    msg.result = enrichError(msg.result);
  }

  if (msg.id !== undefined) pending.delete(msg.id);

  log("<<<", msg);
  process.stdout.write(JSON.stringify(msg) + "\n");
}));

// --- Cleanup ---

child.on("exit", (code) => {
  log("---", { event: "child-exit", code });
  process.exit(code ?? 1);
});

process.on("SIGINT", () => child.kill("SIGINT"));
process.on("SIGTERM", () => child.kill("SIGTERM"));
process.stdin.on("end", () => child.stdin.end());
