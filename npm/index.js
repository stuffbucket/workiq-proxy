"use strict";

// Programmatic interface for workiq-proxy.
//
// Pipe-based (no port, no network):
//   const { createClient } = require("@stuffbucket/workiq-proxy");
//   const client = await createClient();
//   const answer = await client.ask("What's on my calendar?");
//   await client.close();
//
// HTTP server (OpenAI-compatible API):
//   const { createServer } = require("@stuffbucket/workiq-proxy");
//   const server = await createServer();          // starts serve --json
//   const models = await server.listModels();
//   const reply  = await server.chat("Hello!");
//   await server.close();

const { spawn } = require("child_process");
const { createInterface } = require("readline");
const path = require("path");
const os = require("os");
const fs = require("fs");

const pkg = require("./package.json");
const config = pkg.nativeBinary;

/**
 * Resolve the path to the native binary for the current platform.
 * @returns {string} Absolute path to the workiq-proxy binary.
 */
function binaryPath() {
  const key = `${os.platform()}-${os.arch()}`;
  const binary = config.platforms[key];
  if (!binary) {
    throw new Error(`${pkg.name}: unsupported platform ${key}`);
  }
  const p = path.join(__dirname, config.binDir, binary);
  if (!fs.existsSync(p)) {
    throw new Error(
      `${pkg.name}: native binary not found at ${p}\n` +
        `Re-install with: npm install ${pkg.name}`
    );
  }
  return p;
}

/**
 * Build the --workiq-cmd flag value by resolving @microsoft/workiq.
 * Returns an args array fragment, or [] if resolution fails.
 */
function resolveWorkiqArgs() {
  if (!config.resolve) return [];
  try {
    const resolved = require.resolve(config.resolve.module);
    const q = (s) => (s.includes(" ") ? `"${s}"` : s);
    const cmd = config.resolve.prefix
      ? `${q(process.execPath)} ${q(resolved)}`
      : q(resolved);
    return [config.resolve.flag, cmd];
  } catch {
    return [];
  }
}

// ── Pipe-based client (no network) ──────────────────────────────────

/**
 * Unwrap a Work IQ response envelope if present.
 * The upstream sometimes returns {"response":"...","conversationId":"..."}
 * instead of plain text.
 */
function unwrapResponseText(text) {
  if (typeof text !== "string" || !text.startsWith("{")) return text;
  try {
    const obj = JSON.parse(text);
    if (typeof obj.response === "string") return obj.response;
  } catch { /* not JSON */ }
  return text;
}

/**
 * Start workiq-proxy in MCP mode over stdio pipes (no port needed).
 *
 * @param {object} [opts]
 * @param {string} [opts.workiqCmd]     Override --workiq-cmd.
 * @param {boolean} [opts.noLog=false]  Disable disk logging.
 * @returns {Promise<Client>} Resolves after MCP handshake completes.
 */
async function createClient(opts = {}) {
  const bin = binaryPath();

  const args = [];
  if (opts.workiqCmd) {
    args.push("--workiq-cmd", opts.workiqCmd);
  } else {
    args.push(...resolveWorkiqArgs());
  }
  if (opts.noLog) args.push("--no-log");
  // No subcommand → MCP proxy mode (stdio).

  const child = spawn(bin, args, {
    stdio: ["pipe", "pipe", "inherit"],
  });

  const rl = createInterface({ input: child.stdout });
  const pending = new Map(); // id → { resolve, reject }
  let nextId = 0;

  rl.on("line", (line) => {
    let msg;
    try { msg = JSON.parse(line); } catch { return; }
    if (msg.id != null && pending.has(String(msg.id))) {
      const { resolve, reject } = pending.get(String(msg.id));
      pending.delete(String(msg.id));
      if (msg.error) {
        reject(new Error(`${msg.error.message} (code ${msg.error.code})`));
      } else {
        resolve(msg.result);
      }
    }
  });

  function send(method, params) {
    const id = ++nextId;
    const msg = JSON.stringify({ jsonrpc: "2.0", id, method, params });
    child.stdin.write(msg + "\n");
    return new Promise((resolve, reject) => {
      pending.set(String(id), { resolve, reject });
    });
  }

  function notify(method) {
    const msg = JSON.stringify({ jsonrpc: "2.0", method });
    child.stdin.write(msg + "\n");
  }

  // MCP handshake.
  await send("initialize", {
    protocolVersion: "2025-11-25",
    capabilities: {},
    clientInfo: { name: "workiq-proxy-js", version: pkg.version },
  });
  notify("notifications/initialized");

  return new Client(child, rl, send, pending);
}

class Client {
  constructor(child, rl, send, pending) {
    this._child = child;
    this._rl = rl;
    this._send = send;
    this._pending = pending;
  }

  /**
   * Ask a question via the ask_work_iq MCP tool.
   * @param {string} question
   * @returns {Promise<string>} The answer text.
   */
  async ask(question) {
    const result = await this._send("tools/call", {
      name: "ask_work_iq",
      arguments: { question },
    });
    return Client._extractText(result);
  }

  /**
   * Search emails via the search_emails synthetic tool.
   * @param {object} params  { from?, subject?, keywords?, date_range? }
   * @returns {Promise<string>}
   */
  async searchEmails(params) {
    const result = await this._send("tools/call", {
      name: "search_emails",
      arguments: params,
    });
    return Client._extractText(result);
  }

  /**
   * Search documents via the search_documents synthetic tool.
   * @param {object} params  { filename?, keywords?, site?, file_type? }
   * @returns {Promise<string>}
   */
  async searchDocuments(params) {
    const result = await this._send("tools/call", {
      name: "search_documents",
      arguments: params,
    });
    return Client._extractText(result);
  }

  /**
   * Search chats via the search_chats synthetic tool.
   * @param {object} params  { person?, keywords?, date_range?, channel? }
   * @returns {Promise<string>}
   */
  async searchChats(params) {
    const result = await this._send("tools/call", {
      name: "search_chats",
      arguments: params,
    });
    return Client._extractText(result);
  }

  /**
   * Call any MCP tool by name.
   * @param {string} name     Tool name.
   * @param {object} [args]   Tool arguments.
   * @returns {Promise<string>}
   */
  async callTool(name, args = {}) {
    const result = await this._send("tools/call", {
      name,
      arguments: args,
    });
    return Client._extractText(result);
  }

  /**
   * List available tools.
   * @returns {Promise<object>} MCP tools/list result.
   */
  async listTools() {
    return this._send("tools/list", {});
  }

  /**
   * Extract text from an MCP tools/call result.
   */
  static _extractText(result) {
    if (!result || !result.content) return "";
    const parts = result.content
      .filter((c) => c.type === "text")
      .map((c) => unwrapResponseText(c.text))
      .filter(Boolean);
    if (result.isError) return `Error: ${parts.join("\n")}`;
    return parts.join("\n");
  }

  /**
   * Gracefully shut down the client.
   * @returns {Promise<void>}
   */
  async close() {
    return new Promise((resolve) => {
      this._child.on("exit", () => {
        this._rl.close();
        resolve();
      });
      this._child.stdin.end();
    });
  }
}

// ── HTTP server mode ────────────────────────────────────────────────

/**
 * Start a workiq-proxy server in JSON mode and return a handle.
 *
 * @param {object} [opts]
 * @param {number} [opts.port=11435]    Port to listen on.
 * @param {string} [opts.workiqCmd]     Override --workiq-cmd.
 * @param {boolean} [opts.noLog=false]  Disable disk logging.
 * @returns {Promise<Server>} Resolves once the server is listening.
 */
async function createServer(opts = {}) {
  const bin = binaryPath();

  const args = [];
  // Resolve @microsoft/workiq unless the caller supplied a custom command.
  if (opts.workiqCmd) {
    args.push("--workiq-cmd", opts.workiqCmd);
  } else {
    args.push(...resolveWorkiqArgs());
  }
  if (opts.noLog) args.push("--no-log");
  if (opts.port) args.push("--port", String(opts.port));
  args.push("--json", "serve");

  const child = spawn(bin, args, {
    stdio: ["ignore", "pipe", "pipe"],
  });

  // Parse JSONL events from stderr.
  const rl = createInterface({ input: child.stderr });
  const events = [];
  const listeners = [];

  rl.on("line", (line) => {
    let evt;
    try {
      evt = JSON.parse(line);
    } catch {
      return; // skip non-JSON lines (e.g. child stderr bleed)
    }
    events.push(evt);
    for (const fn of listeners) fn(evt);
  });

  // Wait for the "listening" event so we know the address.
  let settled = false;
  const info = await new Promise((resolve, reject) => {
    const onEvent = (evt) => {
      if (evt.msg === "listening") {
        settled = true;
        listeners.splice(listeners.indexOf(onEvent), 1);
        resolve(evt);
      }
    };
    listeners.push(onEvent);

    child.on("error", (err) => reject(err));
    child.on("exit", (code) => {
      if (!settled) reject(new Error(`workiq-proxy exited with code ${code}`));
    });
  });

  const base = `http://${info.addr}`;

  return new Server(child, rl, base, events, listeners);
}

class Server {
  constructor(child, rl, base, events, listeners) {
    this._child = child;
    this._rl = rl;
    this._base = base;
    this._events = events;
    this._listeners = listeners;
  }

  /** Base URL of the running server, e.g. "http://127.0.0.1:11435". */
  get url() {
    return this._base;
  }

  /**
   * Subscribe to JSONL events from the server's stderr.
   * @param {function} fn Called with each parsed event object.
   * @returns {function} Unsubscribe function.
   */
  onEvent(fn) {
    this._listeners.push(fn);
    return () => {
      const i = this._listeners.indexOf(fn);
      if (i !== -1) this._listeners.splice(i, 1);
    };
  }

  /**
   * List available models.
   * @returns {Promise<object>} OpenAI-compatible /v1/models response.
   */
  async listModels() {
    const res = await fetch(`${this._base}/v1/models`);
    if (!res.ok) throw new Error(`listModels: ${res.status} ${res.statusText}`);
    return res.json();
  }

  /**
   * Send a chat completion request.
   *
   * @param {string|object} messageOrBody  A string (used as a single user message)
   *   or a full OpenAI-compatible request body.
   * @param {object} [opts]
   * @param {boolean} [opts.stream=false]  If true, return a ReadableStream of SSE chunks.
   * @returns {Promise<object|ReadableStream>}
   */
  async chat(messageOrBody, opts = {}) {
    const body =
      typeof messageOrBody === "string"
        ? { model: "workiq", messages: [{ role: "user", content: messageOrBody }] }
        : messageOrBody;

    if (opts.stream) body.stream = true;

    const res = await fetch(`${this._base}/v1/chat/completions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(`chat: ${res.status} ${text}`);
    }

    if (opts.stream) return res.body;
    return res.json();
  }

  /**
   * Return all captured JSONL events so far.
   * @returns {object[]}
   */
  get events() {
    return this._events.slice();
  }

  /**
   * Gracefully shut down the server.
   * @returns {Promise<void>}
   */
  async close() {
    return new Promise((resolve) => {
      this._child.on("exit", () => {
        this._rl.close();
        resolve();
      });
      // On Windows, SIGTERM is not supported — kill() calls TerminateProcess.
      // On Unix, SIGTERM allows the server to shut down gracefully.
      this._child.kill();
    });
  }
}

module.exports = { createClient, createServer, binaryPath };
