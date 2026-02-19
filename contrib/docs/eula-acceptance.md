# Work IQ EULA and Authentication

There are two gates between installing Work IQ and making a successful query through MCP. They are separate steps with different mechanisms:

1. **EULA acceptance** — a legal consent gate. Stores acceptance state in `~/.work-iq-cli/`. Does not authenticate.
2. **Microsoft Entra ID authentication** — an OAuth browser flow. Caches tokens to `~/.work-iq-cli/msal_token_cache.dat`. Only triggered on the first actual data query.

Both gates must be cleared before `ask_work_iq` will return data. If either is skipped, the first MCP call will need to handle it — which is problematic because the MCP server runs as a stdio child process where browser flows and consent prompts are awkward at best.

## How the EULA gate works

The MCP server exposes two tools: `accept_eula` and `ask_work_iq`. Calling `ask_work_iq` before accepting the EULA returns an error. The `accept_eula` tool is annotated with `destructiveHint: true`, which signals MCP clients to prompt the user for confirmation before executing — preventing an AI assistant from silently accepting a legal agreement on the user's behalf.

## How the auth gate works

Authentication uses MSAL.NET (confirmed by the maintainer). The binary is a public client application using delegated permissions. When a query requires authentication:

1. MSAL tries `AcquireTokenSilent()` — checks `~/.work-iq-cli/msal_token_cache.dat` for a valid token.
2. If no cached token (first use, or refresh token expired), MSAL calls `AcquireTokenInteractive()`:
   - Starts a localhost HTTP listener on a random port
   - Opens the system browser (`open` on macOS, `xdg-open` on Linux) to the Entra ID login page
   - **Blocks the stdio pipe** while waiting for the OAuth redirect callback
   - User signs in, browser redirects to `http://localhost:{port}/?code=...`
   - Exchanges the auth code for access + refresh tokens, caches them
   - Resumes processing and returns the result over stdio

**Why this matters for MCP:** When the server runs as a child process of Claude Code, the stdio pipe is the only communication channel. While the binary waits for the browser callback, the pipe is frozen. Claude Code may time out the MCP call before the user finishes signing in. This is why pre-authenticating from the terminal is important — it caches the tokens so the MCP server never needs to open a browser.

### Token lifecycle

| Token | Lifetime | What happens on expiry |
|-------|----------|----------------------|
| Access token | ~1 hour | Silently refreshed using refresh token — invisible to MCP client |
| Refresh token | ~90 days (sliding) | Triggers interactive browser flow again |

For day-to-day use, token refresh is invisible. The browser reappears only if: the refresh token expires (~90 days idle), an admin revokes sessions, a Conditional Access policy changes, or the cache file is deleted.

### Auth limitations

- No device-code flow — can't authenticate over SSH or headless environments ([#46](https://github.com/microsoft/work-iq-mcp/issues/46))
- `BROWSER` env var ignored — uses OS tools directly ([#43](https://github.com/microsoft/work-iq-mcp/issues/43))
- Edge may be required as default browser on macOS ([#23](https://github.com/microsoft/work-iq-mcp/issues/23))
- Token cache is likely plaintext on macOS/Linux (MSAL.NET uses DPAPI on Windows only)

### Required delegated permissions (admin-consented)

| Permission | Description |
|-----------|-------------|
| `Sites.Read.All` | Read items in all site collections |
| `Mail.Read` | Read user mail |
| `People.Read.All` | Read all users' relevant people lists |
| `OnlineMeetingTranscript.Read.All` | Read all transcripts of online meetings |
| `Chat.Read` | Read user chat messages |
| `ChannelMessage.Read.All` | Read all channel messages |
| `ExternalItem.Read.All` | Read external items |

Admin consent URL: `https://login.microsoftonline.com/{tenant-id}/adminconsent?client_id=ba081686-5d24-4bc6-a0d6-d034ecffed87`

## Three ways to accept

### 1. CLI pre-acceptance (recommended)

```bash
# If installed globally:
workiq accept-eula

# If using npx:
npx -y @microsoft/workiq accept-eula
```

Accept once from the terminal before configuring the MCP server. The state persists to `~/.work-iq-cli/` and is shared between CLI and MCP modes. This is the cleanest path — the MCP integration works from the first query without the AI needing to negotiate consent.

### 2. MCP tool call from within an AI assistant

The AI assistant calls the `accept_eula` tool with the required parameter:

```json
{
  "eulaUrl": "https://github.com/microsoft/work-iq-mcp"
}
```

Because `destructiveHint: true` is set, the client prompts the user before executing:

| Client | What the user sees |
|--------|-------------------|
| Claude Code (CLI) | Terminal prompt to approve the tool call |
| Claude Code (VS Code) | Approval dialog in the editor |
| VS Code (GitHub Copilot) | Confirmation dialog |
| GitHub Copilot CLI | Inline first-use consent prompt |

This works but is awkward — the AI has to explain a legal agreement mid-conversation, and the user has to understand what they're approving in a tool-call confirmation dialog that wasn't designed for legal consent.

### 3. Interactive CLI session

```bash
workiq ask
```

Running the interactive REPL without a question triggers EULA acceptance as part of the first-use flow.

## State persistence

| Item | Location |
|------|----------|
| EULA acceptance | `~/.work-iq-cli/` (macOS/Linux), `%USERPROFILE%\.work-iq-cli\` (Windows) |
| Auth tokens | `~/.work-iq-cli/msal_token_cache.dat` |

- EULA state is shared between CLI and MCP modes
- Deleting `~/.work-iq-cli/` resets everything (EULA + auth)
- The exact filename tracking EULA acceptance is not publicly documented (the binary is proprietary)

## Recommendation

Clear both gates from the terminal before configuring the MCP server:

```bash
# Step 1: Accept the EULA (stores consent state)
workiq accept-eula

# Step 2: Make a real query (triggers browser auth, caches tokens)
workiq ask -q "What's on my calendar?"
```

Step 1 handles the legal gate. Step 2 forces the browser-based OAuth flow to happen in a normal terminal context — where the browser opens cleanly, you sign in at your own pace, and the tokens get cached. If you skip step 2, the first `ask_work_iq` call through MCP is the one that triggers the browser flow, which blocks the stdio pipe and risks a timeout from Claude Code.

After both steps, the MCP server reuses the cached tokens silently. No browser pop-ups, no consent prompts, no stdio blocking.
