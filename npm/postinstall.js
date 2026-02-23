#!/usr/bin/env node
"use strict";

// Generic postinstall binary downloader.
// All project-specific values come from package.json.

const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const os = require("os");

const pkg = require("./package.json");
const config = pkg.nativeBinary;
const VERSION = pkg.version;

const key = `${os.platform()}-${os.arch()}`;
const binary = config.platforms[key];

if (!binary) {
  console.error(`${pkg.name}: unsupported platform ${key}`);
  process.exit(1);
}

const binDir = path.join(__dirname, config.binDir);
const dest = path.join(binDir, binary);

if (fs.existsSync(dest)) {
  process.exit(0);
}

const url = config.downloadURL
  .replace("{{version}}", VERSION)
  .replace("{{binary}}", binary);

fs.mkdirSync(binDir, { recursive: true });

function download(downloadUrl, downloadDest, redirects) {
  if (redirects > 5) {
    console.error(`${pkg.name}: too many redirects`);
    process.exit(1);
  }

  const client = downloadUrl.startsWith("https") ? https : http;
  client.get(downloadUrl, (res) => {
    if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
      download(res.headers.location, downloadDest, redirects + 1);
      return;
    }
    if (res.statusCode !== 200) {
      console.error(`${pkg.name}: download failed (HTTP ${res.statusCode}) from ${downloadUrl}`);
      console.error(`You can download the binary manually from the releases page.`);
      process.exit(1);
    }
    const file = fs.createWriteStream(downloadDest);
    res.pipe(file);
    file.on("finish", () => {
      file.close();
      fs.chmodSync(downloadDest, 0o755);
    });
  }).on("error", (err) => {
    console.error(`${pkg.name}: download failed: ${err.message}`);
    console.error(`You can download the binary manually from the releases page.`);
    process.exit(1);
  });
}

console.log(`${pkg.name}: downloading ${binary} v${VERSION}...`);
download(url, dest, 0);
