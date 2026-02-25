---
draft: false
title: Tools
description: MCP tools exposed by workiq-proxy.
sidebar:
  order: 2
---

workiq-proxy exposes 9 tools to MCP clients: 2 from the upstream Work IQ server and 7 synthetic search tools added by the proxy.

## Upstream tools

### `accept_eula`

Accepts the End User License Agreement. Must be called before `ask_work_iq` functions.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `eulaUrl` | string | yes | Must be `https://github.com/microsoft/work-iq-mcp` |

This tool has `destructiveHint: true` — MCP clients will prompt for user confirmation.

### `ask_work_iq`

Sends a natural language question to the M365 Copilot Chat API.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `question` | string | yes | Free-form natural language query |

## Synthetic search tools

The proxy exposes 7 structured search tools backed by `ask_work_iq`. These give AI models explicit parameters instead of requiring them to compose natural language queries.

### `search_emails`

Search your Microsoft 365 emails.

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | string | Sender name or email address |
| `subject` | string | Subject line keywords |
| `keywords` | string | Body content keywords |
| `date_range` | string | Time period (e.g., "last week", "yesterday") |

### `search_documents`

Search SharePoint and OneDrive documents.

| Parameter | Type | Description |
|-----------|------|-------------|
| `filename` | string | Filename or partial filename |
| `keywords` | string | Document content keywords |
| `site` | string | SharePoint site name |
| `file_type` | string | File type filter (docx, xlsx, pdf) |

### `search_chats`

Search Teams chat messages.

| Parameter | Type | Description |
|-----------|------|-------------|
| `person` | string | Person name to search chats with |
| `keywords` | string | Chat message keywords |
| `date_range` | string | Time period |

### `search_channels`

Search Teams channel messages.

| Parameter | Type | Description |
|-----------|------|-------------|
| `channel` | string | Channel name |
| `team` | string | Team name |
| `keywords` | string | Message keywords |
| `date_range` | string | Time period |

### `search_meetings`

Search meetings and meeting transcripts.

| Parameter | Type | Description |
|-----------|------|-------------|
| `organizer` | string | Meeting organizer name |
| `subject` | string | Meeting subject keywords |
| `date_range` | string | Time period |

### `search_people`

Search for people in your organization.

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Person's name |
| `department` | string | Department |
| `project` | string | Associated project or topic |

### `search_external`

Search external connector items (Graph connectors).

| Parameter | Type | Description |
|-----------|------|-------------|
| `keywords` | string | Search keywords |
| `source` | string | External source or connector name |

## How synthetic tools work

When an MCP client calls a synthetic tool, the proxy:

1. Reads the structured parameters
2. Composes a natural language question
3. Forwards it to `ask_work_iq`
4. Returns the response

This means all 7 search tools share the same M365 Copilot backend — they just provide better structure for AI models to express intent.
