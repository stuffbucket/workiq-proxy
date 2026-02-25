#!/usr/bin/env node
"use strict";

// Postinstall: download the native binary, then offer EULA acceptance.
// All project-specific values come from package.json.

const https = require("https");
const crypto = require("crypto");
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

  if (!downloadUrl.startsWith("https://")) {
    console.error(`${pkg.name}: refusing non-HTTPS download URL: ${downloadUrl}`);
    process.exit(1);
  }

  https.get(downloadUrl, (res) => {
    if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
      download(res.headers.location, downloadDest, redirects + 1);
      return;
    }
    if (res.statusCode !== 200) {
      console.error(`${pkg.name}: download failed (HTTP ${res.statusCode}) from ${downloadUrl}`);
      console.error(`You can download the binary manually from the releases page.`);
      console.error(`The package is installed but the native binary is missing.`);
      process.exit(0); // Don't break npm install — run.js falls back to npx.
      return;
    }
    const file = fs.createWriteStream(downloadDest);
    res.pipe(file);
    file.on("finish", () => {
      file.close();
      verifyChecksum(downloadDest, () => {
        fs.chmodSync(downloadDest, 0o755);
        offerEula();
      });
    });
  }).on("error", (err) => {
    console.error(`${pkg.name}: download failed: ${err.message}`);
    console.error(`You can download the binary manually from the releases page.`);
    console.error(`The package is installed but the native binary is missing.`);
    process.exit(0); // Don't break npm install — run.js falls back to npx.
  });
}

// Verify the downloaded binary against checksums.txt from the same release.
// This is best-effort — if the checksum file can't be fetched, we warn but
// don't fail the install.
function verifyChecksum(filePath, onSuccess) {
  const checksumsUrl = config.downloadURL
    .replace("{{version}}", VERSION)
    .replace("{{binary}}", "checksums.txt");

  fetchText(checksumsUrl, (err, body) => {
    if (err) {
      console.error(`${pkg.name}: warning: could not fetch checksums.txt (${err.message})`);
      console.error(`${pkg.name}: skipping integrity check`);
      onSuccess();
      return;
    }

    const expected = parseChecksums(body, binary);
    if (!expected) {
      console.error(`${pkg.name}: warning: ${binary} not found in checksums.txt`);
      console.error(`${pkg.name}: skipping integrity check`);
      onSuccess();
      return;
    }

    const actual = sha256File(filePath);
    if (actual !== expected) {
      console.error(`${pkg.name}: CHECKSUM MISMATCH for ${binary}`);
      console.error(`  expected: ${expected}`);
      console.error(`  actual:   ${actual}`);
      console.error(`${pkg.name}: deleting corrupted binary`);
      try { fs.unlinkSync(filePath); } catch { /* ignore */ }
      process.exit(0); // Don't break npm install, but don't run the bad binary.
      return;
    }

    console.log(`${pkg.name}: checksum verified`);
    onSuccess();
  });
}

function sha256File(filePath) {
  const data = fs.readFileSync(filePath);
  return crypto.createHash("sha256").update(data).digest("hex");
}

function parseChecksums(text, filename) {
  for (const line of text.split("\n")) {
    const parts = line.trim().split(/\s+/);
    if (parts.length >= 2 && parts[1] === filename) {
      return parts[0].toLowerCase();
    }
  }
  return null;
}

function fetchText(fetchUrl, cb, redirects) {
  if ((redirects || 0) > 5) {
    cb(new Error("too many redirects"));
    return;
  }
  if (!fetchUrl.startsWith("https://")) {
    cb(new Error(`refusing non-HTTPS URL: ${fetchUrl}`));
    return;
  }
  https.get(fetchUrl, (res) => {
    if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
      fetchText(res.headers.location, cb, (redirects || 0) + 1);
      return;
    }
    if (res.statusCode !== 200) {
      cb(new Error(`HTTP ${res.statusCode}`));
      return;
    }
    let body = "";
    res.on("data", (d) => { body += d; });
    res.on("end", () => cb(null, body));
  }).on("error", cb);
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
