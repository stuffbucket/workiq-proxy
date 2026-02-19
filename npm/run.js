#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const os = require("os");

const PLATFORM_MAP = {
  "darwin-x64": "workiq-proxy-darwin-x64",
  "darwin-arm64": "workiq-proxy-darwin-arm64",
  "linux-x64": "workiq-proxy-linux-x64",
  "linux-arm64": "workiq-proxy-linux-arm64",
  "win32-x64": "workiq-proxy-win-x64.exe",
  "win32-arm64": "workiq-proxy-win-arm64.exe",
};

const key = `${os.platform()}-${os.arch()}`;
const binary = PLATFORM_MAP[key];
if (!binary) {
  console.error(`workiq-proxy: unsupported platform ${key}`);
  process.exit(1);
}

const binPath = path.join(__dirname, "bin", binary);
try {
  execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  if (err.status != null) {
    process.exit(err.status);
  }
  console.error(`workiq-proxy: failed to run ${binPath}: ${err.message}`);
  process.exit(1);
}
