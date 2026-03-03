---
# AUTO-GENERATED — do not edit by hand.
# Run "make docs" or "go generate ./cmd/workiq-proxy/" to regenerate.
title: Error Messages
description: Documented error messages from workiq-proxy, with explanations and fixes.
sidebar:
  order: 11
---

workiq-proxy uses structured error messages that explain **what happened**,
**why it matters**, and **what to do next**.

## Could not find the Work IQ CLI ({name}).

**Why:** Without it, workiq-proxy has nothing to connect to.

**Fix:** Install it with: npm install -g @microsoft/workiq

---

## Could not start the backend process.

**Why:** Without the backend, workiq-proxy cannot answer queries.

**Fix:** Make sure the Work IQ CLI is installed: npm install -g @microsoft/workiq

---

## The backend did not respond during startup.

**Why:** This usually means the Work IQ CLI is installed but not working.

**Fix:** Try reinstalling: npm install -g @microsoft/workiq

---

## The backend process stopped unexpectedly.

**Why:** Without it, workiq-proxy can no longer process requests.

**Fix:** Restart the server to reconnect.

---

## Could not find an available port.

**Why:** The server needs a port to listen on, and the default range is occupied.

**Fix:** Close other programs or try a different port: workiq-proxy serve --port 12000

---

## The server stopped unexpectedly.

**Why:** Any connected clients have been disconnected.

**Fix:** Check the error above and restart: workiq-proxy serve

---

