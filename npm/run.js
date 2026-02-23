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
const args = process.argv.slice(2);

if (config.resolve) {
  try {
    const resolved = require.resolve(config.resolve.module);
    const cmd = config.resolve.prefix
      ? `${process.execPath} ${resolved}`
      : resolved;
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
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 1);
  }
});
