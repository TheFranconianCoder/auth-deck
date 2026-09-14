# AuthDeck

A lightweight OAuth 2.0 local token proxy that acts as a bridge between API clients and multiple external identity
providers. AuthDeck presents itself as a single OAuth2 endpoint, so tools never need to know which provider issues a
token—or where the client secrets live.

- **Ad-hoc token selection**—a terminal UI lets you decide which provider fulfills a request.
- **Transparent proxying**—forward requests to upstream APIs with a valid bearer token injected automatically.
- **Works with anything**—Bruno, curl, scripts. No client changes required.
- **Clean Architecture**—entities, use cases, state, and adapters, standard library first.

---

## Table of Contents

- [How it works](#how-it-works)
- [Install & run](#install--run)
- [Configuration](#configuration)
- [Endpoints](#endpoints)
- [TUI](#tui)
- [Token lifecycle](#token-lifecycle)
- [Examples](#examples)
- [Token store](#token-store)
- [Architecture](#architecture)
- [Security notes](#security-notes)

---

## How it works

AuthDeck exposes a single, simple ingress: **OAuth 2.0 Client Credentials**. Whatever the upstream provider requires—a
machine-to-machine exchange or a full interactive browser login—is handled inside AuthDeck and never leaks to the
client.

```
        ┌──────────────┐  POST /token   ┌──────────────┐   token request   ┌──────────────┐
        │  API client  │───────────────▶│   AuthDeck   │──────────────────▶│ Identity     │
        │ (Bruno/curl) │                │  (localhost) │◀──────────────────│ Provider     │
        └──────────────┘                └──────┬───────┘   access token    └──────────────┘
               ▲                               │
               │        TUI provider choice    │
               └───────────────────────────────┘
```

Two ways to use it:

1. **Token endpoint**—the client asks AuthDeck for a token and uses it itself (`POST /token`).
2. **Reverse proxy**—the client sends the real API request to AuthDeck and never touches a token (`/proxy/*`).

Provider selection happens in the TUI unless it is pinned in the path. A provider can be given in the token path
(`/token/{provider}`) or the proxy path (`/direct/{provider}/...`); then the selection is skipped. Pending requests can
also be rejected outright, which returns `403` to the caller.

> **Interactive upstreams need no special client flow.** Selecting a provider configured with `flow: authorization_code`
> makes AuthDeck open the browser, complete the login, and return the token to the client—all behind the same
> `POST /token` call. The client still speaks only Client Credentials.

---

## Install & run

Requires **Go 1.27+**.

### go install

```bash
go install github.com/TheFranconianCoder/auth-deck/cmd/auth-deck@latest
```

This places the binary in `$(go env GOPATH)/bin`—make sure that directory is on your `PATH`.

### mise

```bash
mise use -g go@1.27 go:github.com/TheFranconianCoder/auth-deck/cmd/auth-deck@latest
```

mise installs the binary into its own tool directory and manages `PATH` for you.

### From source

```bash
git clone https://github.com/TheFranconianCoder/auth-deck
cd auth-deck
go run ./cmd/auth-deck
```

### Configure and run

All install methods read the same config file. Create it once, then start the binary:

```bash
mkdir -p ~/.config/auth-deck
curl -fsSL https://raw.githubusercontent.com/TheFranconianCoder/auth-deck/main/config.example.yaml \
  -o ~/.config/auth-deck/config.yaml   # then edit your providers
auth-deck
```

The proxy listens on `127.0.0.1:9090` by default and the TUI starts automatically. Press `ctrl+c` to stop both.

### Version

Installs via `go install` and mise embed the module version automatically—no build flags needed. The TUI shows it in
the title bar, and `auth-deck -version` prints it alongside the Go version and, for local builds, the commit. For
readable versions, publish SemVer tags (`git tag vX.Y.Z && git push origin vX.Y.Z`); untagged installs fall back to a
pseudo-version, and local `go build`/`go run` show the commit hash (or `dev`).

---

## Configuration

AuthDeck is configured through a single YAML file. When no `-config` flag is given, the file is read from the OS config
directory: `~/.config/auth-deck/config.yaml` on Linux, `~/Library/Application Support/auth-deck/config.yaml` on macOS,
and `%AppData%\auth-deck\config.yaml` on Windows. Override with `-config`. Values may reference environment variables
using `${VAR}` syntax—useful for secrets.

```yaml
server:
  addr: "127.0.0.1:9090"
  # Optional. Default per OS (see "Token store").
  # token_store: "~/.local/share/auth-deck/tokens.json"

providers:
  logto-m2m:
    client_id: "your-m2m-app-id"
    client_secret: "${LOGTO_M2M_SECRET}"
    token_url: "https://example.logto.app/oidc/token"
    scopes: ["all"]
    resource: "https://example.logto.app/api"
    base_url: "https://example.logto.app"
    flow: "client_credentials"

  logto:
    client_id: "your-spa-or-web-app-id"
    client_secret: "${LOGTO_SECRET}"
    auth_url: "https://example.logto.app/oidc/auth"
    token_url: "https://example.logto.app/oidc/token"
    redirect_uri: "http://127.0.0.1:9090/callback"
    scopes: ["openid", "profile", "offline_access"]
    base_url: "https://example.logto.app"
    flow: "authorization_code"
    pkce: true
    prompt: "consent"
```

### Server options

| Key | Default | Description |
|---|---|---|
| `server.addr` | `127.0.0.1:9090` | Listen address. |
| `server.token_store` | OS-specific | Path to the persisted token file (see below). |

### Provider options

| Key | Required | Default | Description |
|---|---|---|---|
| `client_id` | yes | — | OAuth client ID. |
| `client_secret` | yes | — | OAuth client secret (supports `${ENV}`). |
| `token_url` | yes | — | Token endpoint. |
| `auth_url` | for `authorization_code` | — | Authorization endpoint. |
| `redirect_uri` | no | `http://127.0.0.1:9090/callback` | Registered callback URL. |
| `scopes` | no | `[]` | Requested scopes. |
| `base_url` | for `/proxy` | — | Upstream API base URL. |
| `flow` | no | `client_credentials` | `client_credentials` or `authorization_code`. |
| `resource` | no | — | `resource` parameter (e.g. Logto Management API). |
| `token_path` | no | `access_token` | Dotted JSON path to the access token in the response. |
| `token_type_path` | no | `token_type` | Dotted path to the token type. |
| `expires_path` | no | `expires_in` | Dotted path to the lifetime (seconds). |
| `pkce` | no | `true` | Use PKCE (S256) for `authorization_code`. |
| `prompt` | no | — | `prompt` parameter for the auth request (e.g. `consent`). |

Provider order in the TUI follows the order in the YAML file.

> **Custom responses:** Some providers return the token under a non-standard field. Set `token_path`/`token_type_path`/
> `expires_path` accordingly, e.g. `token_path: "data.token"`. There is no implicit fallback: if the configured path is
> missing, the request fails with a clear error.

---

## Endpoints

AuthDeck accepts `client_credentials` and `refresh_token`. A `POST /token` carrying any other `grant_type` is rejected
with `400`. The `refresh_token` grant is a convenience alias for clients that only re-request a token when they hold
one: AuthDeck resolves the provider from the path and returns whatever token it has (or can obtain), exactly as for
`client_credentials`. It therefore requires `POST /token/{provider}`.

| Method | Path | Description |
|---|---|---|
| `POST` | `/token` | Returns a token; provider chosen in the TUI. |
| `POST` | `/token/{provider}` | Returns a token for a specific provider (no TUI). |
| `GET` | `/callback` | Internal OAuth redirect target for interactive upstream flows. |
| `ANY` | `/proxy/*` | Proxies to the upstream API; provider chosen in the TUI. |
| `ANY` | `/direct/{provider}/*` | Proxies to a pinned provider's upstream API (no TUI). |
| `GET` | `/health` | Health check. |

**Token response**

```json
{ "access_token": "eyJ...", "token_type": "Bearer", "expires_in": 3600, "refresh_token": "authdeck:logto-m2m" }
```

The `refresh_token` is a stable, opaque per-provider value, not an upstream secret: the provider is addressed by the
request path, so the value only signals that the client may re-request a token. Pass it back with
`grant_type=refresh_token` to receive a fresh response.

---

## TUI

Providers and the request log sit side by side; pending requests appear below.

```
AuthDeck :: OAuth 2.0 Local Token Proxy

┌ Providers ────────────────────┐ ┌ Log ──────────────────────────────────┐
│ ▸ [1] logto        ● active 59m │ │ 14:30:21 GET   logto  200 /api/users  │
│   [2] logto-m2m    ● active 1h  │ │ 14:30:19 TOKEN logto  200 /token      │
│   [3] google       ○ no token   │ │                                        │
└───────────────────────────────┘ └────────────────────────────────────────┘

New request—select provider:
▸ PROXY /api/users  14:30:21

[1-9,a-z] select  [tab] focus  [↑↓] move  [esc] reject
```

Each log line shows `time`, `method`, `provider`, HTTP `status`, and the **called path** (especially useful for
`/proxy/*` requests), followed by duration or an error.

| Key | Action |
|---|---|
| `1`–`9`, `a`–`z` | With a pending request: choose the provider. With none: **force a fresh token** (refresh, re-mint, or browser login). `1`–`9` cover the first nine providers, `a`–`z` the next twenty-six. |
| `tab` | Switch focus between the provider list and the pending requests. |
| `↑` / `↓` | Move the cursor within the focused list (the provider list scrolls automatically). |
| `enter` | In the provider list: choose the highlighted provider. In the pending list: return focus to the provider list. |
| `esc` | With a pending request: reject it (the caller receives `403`). Otherwise clear the current notice. |
| `ctrl+c` | Quit AuthDeck (stops the proxy too). |

The provider list shows a viewport of twelve entries with a `n-m/total` position in its header. Providers beyond the
first 35 (nine digits plus twenty-six letters) have no shortcut—their label is `[–]`—and are only reachable by pinning
them in the path (`/token/{provider}` or `/direct/{provider}/...`). The focused list has a highlighted border and
cursor; the other list is dimmed until `tab` moves the focus back.

Token markers: `● active` (valid, with remaining time), `◐ expired`, `↻ re-login` (interactive login required),
`○ no token`, `✗ error`.

---

## Token lifecycle

- **Cache:** tokens are kept in memory and persisted to disk (see below). A request reuses a valid cached token.
- **Auto-refresh:** a background loop runs every 30 s (plus once at startup) and renews tokens within 5 minutes of
  expiry:
  - a provider with a refresh token → silent refresh;
  - `client_credentials` → re-mint;
  - `authorization_code` without a usable refresh token → a `↻ re-login` notice instead of a failed request.
- **Manual force:** pressing a provider's number or letter with no pending request always fetches a fresh token
  (`RefreshNow`), bypassing the cache.
- **PKCE:** enabled by default (`S256`) for `authorization_code`; disable with `pkce: false`.

### Refresh tokens (provider-specific)

Providers only return a refresh token when the request asks for it. For Logto/OIDC that means the `offline_access`
scope—and Logto additionally requires `prompt=consent` unless its non-standard “always issue refresh tokens” toggle is
enabled:

```yaml
scopes: ["openid", "profile", "offline_access"]
prompt: "consent"
```

---

## Examples

### Bruno—Client Credentials

```
Grant Type:  Client Credentials
Token URL:   http://127.0.0.1:9090/token          # TUI chooses the provider
             http://127.0.0.1:9090/token/logto-m2m # or pin one
Client ID:   x        # ignored; AuthDeck uses the provider config
Client Secret: x      # ignored
```

That is the only client configuration needed. If the pinned/provider-selected upstream uses `authorization_code`,
AuthDeck handles the browser login transparently and still returns the token here.

AuthDeck's token response includes a `refresh_token`, so clients that only re-request tokens when one is present (such
as Bruno) refresh against the pinned endpoint instead of sitting on an expired token:

```
Grant Type:  Client Credentials
Token URL:   http://127.0.0.1:9090/token/logto-m2m # pin the provider
```

On refresh, the client sends `grant_type=refresh_token` to the same `/token/{provider}` URL with the returned token.
AuthDeck then resolves the provider from the path and returns a current token—identical to a fresh
`client_credentials` call.

### curl—token

```bash
curl -s -X POST http://127.0.0.1:9090/token/logto-m2m

# Refresh (provider pinned in the path):
curl -s -X POST http://127.0.0.1:9090/token/logto-m2m \
  -d grant_type=refresh_token -d refresh_token=authdeck:logto-m2m
```

### curl—transparent proxy

```bash
# Provider pinned in the path (no TUI):
curl -s http://127.0.0.1:9090/direct/logto-m2m/api/users

# Provider chosen in the TUI:
curl -s http://127.0.0.1:9090/proxy/api/users
```

Everything after `/proxy/` or `/direct/{provider}/` is forwarded to `<base_url>/<path>`; method, query, headers, and
body are passed through. All client request headers are forwarded, except hop-by-hop headers, proxy-specific
`X-Forwarded-*` headers, `Content-Length`, `Authorization` (AuthDeck sets its own), and `Host`. AuthDeck adds
`Authorization: Bearer <token>`; upstream response headers are passed back (minus hop-by-hop and `X-Forwarded-*`
headers). As a local client rather than a reverse proxy, AuthDeck sets no forwarding headers of its own.

Because the provider lives in the path, a client only needs to know an IdP or an API base URL—never an AuthDeck
header. Point an OAuth2 client at `/token/{provider}` (a standard client-credentials response), or point the API base
URL at `/direct/{provider}`. For example, with `golang.org/x/oauth2/clientcredentials`:

```go
cfg := clientcredentials.Config{
    ClientID:     "ignored",
    ClientSecret: "ignored",
    TokenURL:     "http://127.0.0.1:9090/token/logto-m2m",
}
httpClient := cfg.Client(ctx) // adds the bearer token and refreshes it automatically
```

---

## Token store

Tokens are persisted so a restart can reuse them (including refresh tokens). The file is written atomically with
restrictive permissions (`0600`, directory `0700`).

| OS | Default path |
|---|---|
| Linux | `$XDG_DATA_HOME/auth-deck/tokens.json`, else `~/.local/share/auth-deck/tokens.json` |
| macOS | `~/Library/Application Support/auth-deck/tokens.json` |
| Windows | `%AppData%\auth-deck\tokens.json` |

Override with `server.token_store` (supports a leading `~`):

```yaml
server:
  token_store: "~/.local/share/auth-deck/tokens.json"
```

---

## Architecture

AuthDeck follows Clean Architecture with clear layer boundaries:

```
cmd/auth-deck/main.go        # composition root (wiring)
internal/
  core/entities/            # Token, Provider—no dependencies
  core/usecases/            # use cases + interfaces.go (ports & DTOs)
  state/                    # in-memory read model: token cache, request queue, auth states
  infrastructure/           # adapters: config, oauth client, browser, file store, forwarder
  api/                      # HTTP adapter (router, handlers)
  tui/                      # terminal UI adapter
  types/                    # AppError + HTTP mapping
```

Ports (`TokenClient`, `TokenStore`, `ProviderCatalog`, `RequestQueue`, `Browser`, `Forwarder`, `Notifier`) are defined
next to the use cases and implemented by the adapters. Use cases never talk HTTP directly.

---

## Security notes

- AuthDeck binds to `127.0.0.1` only and trusts the local user. Do not expose it on a network interface.
- Persisted tokens are stored **in plaintext**. Prefer a private token store path; do not commit it to version control.
- Client secrets should come from the environment (`${VAR}`), not from a committed YAML file.
- The proxy forwards arbitrary upstream paths—keep `base_url` values trusted.
