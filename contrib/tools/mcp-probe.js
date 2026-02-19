#!/usr/bin/env node
//
// mcp-probe.js — Probe any MCP stdio server and dump its capabilities and tool definitions.
//
// Usage:
//   node mcp-probe.js <command> [args...]
//
// Examples:
//   node mcp-probe.js npx -y @microsoft/workiq mcp
//   node mcp-probe.js node my-mcp-server.js
//   node mcp-probe.js python mcp_server.py
//

const { spawn } = require("child_process");

const command = process.argv[2];
const args = process.argv.slice(3);

if (!command) {
  console.error("Usage: node mcp-probe.js <command> [args...]");
  console.error("Example: node mcp-probe.js npx -y @microsoft/workiq mcp");
  process.exit(1);
}

const child = spawn(command, args, { stdio: ["pipe", "pipe", "pipe"] });

let stdout = "";
let stderr = "";
child.stdout.on("data", (d) => { stdout += d.toString(); });
child.stderr.on("data", (d) => { stderr += d.toString(); });

child.on("error", (err) => {
  console.error(`Failed to start: ${err.message}`);
  process.exit(1);
});

function send(msg) {
  child.stdin.write(JSON.stringify(msg) + "\n");
}

function parseResponses(raw) {
  const results = {};
  for (const line of raw.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    try {
      const parsed = JSON.parse(trimmed);
      if (parsed.id !== undefined) {
        results[parsed.id] = parsed;
      }
    } catch {
      // skip non-JSON lines
    }
  }
  return results;
}

// Step 1: initialize
send({
  jsonrpc: "2.0",
  id: 1,
  method: "initialize",
  params: {
    protocolVersion: "2024-11-05",
    capabilities: {},
    clientInfo: { name: "mcp-probe", version: "1.0.0" },
  },
});

setTimeout(() => {
  // Step 2: initialized notification
  send({ jsonrpc: "2.0", method: "notifications/initialized" });

  setTimeout(() => {
    // Step 3: tools/list
    send({ jsonrpc: "2.0", id: 2, method: "tools/list", params: {} });

    // Step 4: resources/list (may return -32601 if unsupported)
    send({ jsonrpc: "2.0", id: 3, method: "resources/list", params: {} });

    // Step 5: prompts/list (may return -32601 if unsupported)
    send({ jsonrpc: "2.0", id: 4, method: "prompts/list", params: {} });

    setTimeout(() => {
      const responses = parseResponses(stdout);

      const report = {
        command: [command, ...args].join(" "),
        probed_at: new Date().toISOString(),
        server_info: null,
        capabilities: null,
        tools: null,
        resources: null,
        prompts: null,
        stderr: stderr.trim() || null,
      };

      // Initialize response
      const init = responses[1];
      if (init?.result) {
        report.server_info = init.result.serverInfo || null;
        report.capabilities = init.result.capabilities || null;
      } else if (init?.error) {
        report.server_info = { error: init.error };
      }

      // Tools response
      const tools = responses[2];
      if (tools?.result) {
        report.tools = tools.result.tools || [];
      } else if (tools?.error) {
        report.tools = { error: tools.error };
      }

      // Resources response
      const resources = responses[3];
      if (resources?.result) {
        report.resources = resources.result.resources || [];
      } else if (resources?.error) {
        report.resources = { unsupported: true, error_code: resources.error.code, message: resources.error.message };
      }

      // Prompts response
      const prompts = responses[4];
      if (prompts?.result) {
        report.prompts = prompts.result.prompts || [];
      } else if (prompts?.error) {
        report.prompts = { unsupported: true, error_code: prompts.error.code, message: prompts.error.message };
      }

      console.log(JSON.stringify(report, null, 2));

      child.kill();
      process.exit(0);
    }, 5000);
  }, 1000);
}, 5000);
