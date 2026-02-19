#!/usr/bin/env node
"use strict";

const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const os = require("os");

const pkg = require("./package.json");
const VERSION = pkg.version;

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

const binDir = path.join(__dirname, "bin");
const dest = path.join(binDir, binary);

if (fs.existsSync(dest)) {
  process.exit(0);
}

const url = `https://github.com/stuffbucket/workiq-proxy/releases/download/v${VERSION}/${binary}`;

fs.mkdirSync(binDir, { recursive: true });

function download(url, dest, redirects) {
  if (redirects > 5) {
    console.error("workiq-proxy: too many redirects");
    process.exit(1);
  }

  const client = url.startsWith("https") ? https : http;
  client.get(url, (res) => {
    if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
      download(res.headers.location, dest, redirects + 1);
      return;
    }
    if (res.statusCode !== 200) {
      console.error(`workiq-proxy: download failed (HTTP ${res.statusCode}) from ${url}`);
      console.error("You can download the binary manually from https://github.com/stuffbucket/workiq-proxy/releases");
      process.exit(1);
    }
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on("finish", () => {
      file.close();
      fs.chmodSync(dest, 0o755);
    });
  }).on("error", (err) => {
    console.error(`workiq-proxy: download failed: ${err.message}`);
    console.error("You can download the binary manually from https://github.com/stuffbucket/workiq-proxy/releases");
    process.exit(1);
  });
}

console.log(`workiq-proxy: downloading ${binary} v${VERSION}...`);
download(url, dest, 0);
