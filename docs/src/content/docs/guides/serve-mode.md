---
draft: false
title: OpenAI-compatible API
description: Use workiq-proxy's serve mode to expose an OpenAI Chat Completions endpoint.
sidebar:
  order: 4
---

workiq-proxy includes a `serve` mode that exposes an OpenAI-compatible Chat Completions API. Any tool that speaks the OpenAI protocol can query your M365 data.

## Start the server

```bash
npx -y @stuffbucket/workiq-proxy serve
```

The server starts on port `11435` by default and seeks the next available port if busy.

### Custom port

```bash
npx -y @stuffbucket/workiq-proxy serve --port 8080
```

## Endpoint

```
POST http://localhost:11435/v1/chat/completions
```

### Request

```json
{
  "model": "workiq",
  "messages": [
    { "role": "user", "content": "What emails did I get today?" }
  ]
}
```

### Response

Standard OpenAI Chat Completions response format with the Work IQ answer in `choices[0].message.content`.

## Use with any OpenAI-compatible client

Point any tool at `http://localhost:11435` as the base URL. No API key required (the server uses your local Work IQ auth tokens).

### Example: curl

```bash
curl http://localhost:11435/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"workiq","messages":[{"role":"user","content":"What meetings do I have tomorrow?"}]}'
```

### Example: Python (openai library)

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:11435/v1", api_key="unused")

response = client.chat.completions.create(
    model="workiq",
    messages=[{"role": "user", "content": "Find my recent documents about budgets"}],
)
print(response.choices[0].message.content)
```
