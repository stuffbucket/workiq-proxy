---
draft: false
title: Admin Consent
description: Delegated permissions and tenant admin consent for Work IQ.
sidebar:
  order: 4
---

workiq-proxy queries Microsoft 365 via delegated permissions. A tenant admin must grant consent before any user in the organization can authenticate.

## Required delegated permissions

| Permission | What it reads |
|-----------|---------------|
| `Mail.Read` | Your email |
| `Chat.Read` | Teams chat messages |
| `ChannelMessage.Read.All` | Teams channel messages |
| `Sites.Read.All` | SharePoint / OneDrive documents |
| `People.Read.All` | Org directory |
| `OnlineMeetingTranscript.Read.All` | Meeting transcripts |
| `ExternalItem.Read.All` | External connector items |

## Admin consent URL

```
https://login.microsoftonline.com/{tenant-id}/adminconsent?client_id=ba081686-5d24-4bc6-a0d6-d034ecffed87
```

Replace `{tenant-id}` with your Microsoft Entra tenant ID.

Once an admin has granted consent, individual users can authenticate by running any data query — the OAuth flow will open automatically in their browser.
