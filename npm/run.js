#!/usr/bin/env node
"use strict";

// Generic npm delivery shim for a native binary.
// All project-specific values come from package.json.

const { spawn } = require("child_process");
const path = require("path");
const os = require("os");

const pkg = require("./package.json");
const config = pkg.nativeBinary;

const key = `${os.platform()}-${os.arch()}`;
const binary = config.platforms[key];
if (!binary) {
  console.error(`${pkg.name}: unsupported platform ${key}`);
  process.exit(1);
}

const binPath = path.join(__dirname, config.binDir, binary);

if (!require("fs").existsSync(binPath)) {
  console.error(
    `${pkg.name}: native binary not found at ${binPath}\n` +
    `This usually means the postinstall download failed.\n` +
    `Re-install with: npm install -g ${pkg.name}\n` +
    `Or download manually from: ${config.downloadURL.replace("{{version}}", pkg.version).replace("{{binary}}", binary)}`
  );
  process.exit(1);
}

const args = process.argv.slice(2);

if (config.resolve) {
  try {
    const resolved = require.resolve(config.resolve.module);
    // Quote each path component so Windows paths with spaces
    // (e.g. C:\Program Files\nodejs\node.exe) survive the Go
    // side's whitespace splitting of --workiq-cmd.
    const q = (p) => (p.includes(" ") ? `"${p}"` : p);
    const cmd = config.resolve.prefix
      ? `${q(process.execPath)} ${q(resolved)}`
      : q(resolved);
    args.unshift(config.resolve.flag, cmd);
  } catch {
    // Fall through — Go binary's default will be used.
  }
}

const child = spawn(binPath, args, { stdio: "inherit" });

// Forward signals so MCP clients that kill the Node PID also stop the Go binary.
for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(sig, () => child.kill(sig));
}

child.on("exit", (code, signal) => {
  if (signal) {
    // On Unix, re-raise with the same signal so the parent sees
    // the correct wait status. On Windows, signals are limited —
    // just exit non-zero.
    try {
      process.kill(process.pid, signal);
    } catch {
      process.exit(1);
    }
  } else {
    process.exit(code ?? 1);
  }
});
