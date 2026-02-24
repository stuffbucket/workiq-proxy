#!/usr/bin/env node
"use strict";

// Postinstall: download the native binary, then offer EULA acceptance.
// All project-specific values come from package.json.

const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const os = require("os");
const { spawnSync } = require("child_process");

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
  offerEula();
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
      offerEula();
    });
  }).on("error", (err) => {
    console.error(`${pkg.name}: download failed: ${err.message}`);
    console.error(`You can download the binary manually from the releases page.`);
    process.exit(1);
  });
}

// After binary download, offer interactive EULA acceptance if possible.
// This is non-fatal — install succeeds regardless.
function offerEula() {
  // Check if EULA is already accepted (state dir exists with content).
  const stateDir = path.join(os.homedir(), ".work-iq-cli");
  if (fs.existsSync(stateDir)) {
    const entries = fs.readdirSync(stateDir);
    if (entries.some((e) => e !== "mcp-traffic.log" && e !== "msal_token_cache.dat")) {
      return; // Likely already accepted.
    }
  }

  const isTTY = process.stdin.isTTY && process.stdout.isTTY;

  if (isTTY) {
    console.log("");
    console.log(`${pkg.name}: Running Work IQ EULA acceptance...`);
    console.log("");
    const result = spawnSync(dest, ["accept-eula"], {
      stdio: "inherit",
    });
    if (result.status === 0) {
      console.log("");
      console.log(`${pkg.name}: EULA accepted. You're ready to go.`);
    }
    // Non-zero exit is fine — user declined or it failed. Install still succeeds.
  } else {
    console.log("");
    console.log(`${pkg.name}: To accept the Work IQ EULA, run:`);
    console.log(`  npx @stuffbucket/workiq-proxy accept-eula`);
  }
}

console.log(`${pkg.name}: downloading ${binary} v${VERSION}...`);
download(url, dest, 0);
