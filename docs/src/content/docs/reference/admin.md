---
draft: false
title: Admin Consent
description: Delegated permissions and tenant admin consent for Work IQ.
sidebar:
  order: 4
---

Before anyone on your team can use workiq-proxy, a tenant admin needs to grant one-time consent for the required Microsoft Graph permissions. This takes about 30 seconds.

workiq-proxy queries Microsoft 365 via **delegated permissions** — it can only access data that the signed-in user already has access to. It does not use application permissions and cannot access other users' data.

## Required delegated permissions

| Permission | What it reads | Why |
|-----------|---------------|-----|
| `Mail.Read` | The user's email | Email search |
| `Chat.Read` | The user's Teams chats | Chat search |
| `ChannelMessage.Read.All` | Teams channel messages | Channel search |
| `Sites.Read.All` | SharePoint / OneDrive documents | Document search |
| `People.Read.All` | Org directory profiles | People search |
| `OnlineMeetingTranscript.Read.All` | Meeting transcripts | Meeting search |
| `ExternalItem.Read.All` | Graph connector items | External search |

All permissions are **read-only**. workiq-proxy never writes, modifies, or deletes any data.

## Admin consent URL

```
https://login.microsoftonline.com/{tenant-id}/adminconsent?client_id=ba081686-5d24-4bc6-a0d6-d034ecffed87
```

Replace `{tenant-id}` with your Microsoft Entra tenant ID.

After granting consent, individual users can authenticate by running any data query — their browser will open to a Microsoft sign-in page automatically.
