---
title: "Security"
description: "What aichteeteapee locks down by default (CORS, WebSocket origins, uploads, static paths, request IDs) and how FuckSecurity opens it up for local dev."
nav_order: 9
---

# Security

Secure by default. You opt out of safety, never into it.

## What is locked down

| Area | Default | Where |
| --- | --- | --- |
| CORS | Browsers receive no cross-origin response access until you allow an origin. | [CORS](middleware.md#cors) |
| WebSocket origin | The `Origin` host must equal the request `Host`. A request without `Origin` (not a browser) is let through. | [Upgrade handler](websocket.md#upgrade-handler) |
| Listen address | Loopback only, `127.0.0.1:8080` and `127.0.0.1:8443`. | [Configuration](server.md#configuration) |
| Static files | Directory listings are off. Serve a directory whose contents and symlinks you control. | [Static files](server.md#static-files) |
| Uploads | The client's file name loses its directories, files are created `0600` and never overwrite an existing file, the request is capped at 100MB. | [FileUploadHandler](server.md#fileuploadhandler) |
| Request IDs | A client `X-Request-ID` longer than 128 characters or outside `[a-zA-Z0-9._-]` is replaced with a fresh UUID. | [RequestID](middleware.md#requestid) |
| Auth | BasicAuth and BearerAuth compare in constant time. Empty bearer tokens never match. | [Middleware](middleware.md#basicauth) |
| Security headers | `nosniff`, `DENY` framing, HSTS and a strict referrer policy, once you add `SecurityHeaders()`. | [SecurityHeaders](middleware.md#securityheaders) |
| Proxy | Upstream bodies are capped at 100MB, hop-by-hop headers are stripped, and nothing private or authenticated is cached. | [Proxy](proxy.md) |
| Client | Responses are capped at 10 MiB and redirects stay on the base URL's origin. | [Client](client.md#security-boundary) |
| Echo | `EchoHandler` leaves `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key` and `Proxy-Authorization` out of its answer. | [EchoHandler](server.md#echohandler) |
| Unix bridge | Socket directory `0700`, sockets `0600`. | [Unix socket bridge](websocket.md#unix-socket-bridge) |

Middleware is opt-in: the server adds none on its own. Put `SecurityHeaders()`, `CORS()` and friends in `GlobalMiddlewares`, as the [quick look](index.md#quick-look) does.

## Local dev mode

For local development, relax CORS and the default WebSocket origin check.

```go
aichteeteapee.FuckSecurity()
```

It flips a process-wide flag. New `CORS()` middleware allows all origins while the flag is on, and the default WebSocket origin check accepts any origin. Call `aichteeteapee.UnfuckSecurity()` to clear it. `aichteeteapee.IsDevMode()` reports its current value.

Rebuild CORS middleware after toggling the flag. The default WebSocket check reads it on every upgrade. A custom WebSocket origin check controls its own policy.

Don't ship it. That's the whole point of the name.

## Per-component overrides

Open up one thing without the global switch:

```go
middleware.CORS(middleware.WithAllowAllOrigins())

wshub.UpgradeHandler(hub,
    wshub.WithUpgradeHandlerCheckOrigin(
        aichteeteapee.GetPermissiveWebSocketCheckOrigin,
    ),
)
```

## What is on you

- Strip or overwrite client-supplied `X-Forwarded-For`, `X-Real-IP` and `X-Forwarded-Proto` at your trusted edge. Request IP and scheme helpers read those headers.
- `prawxxey.ForwardRequest` and `klhayyent.New` treat their URLs as trusted configuration. If a URL comes from a user, run your own SSRF checks first; neither package blocks private addresses.
- Timeout cancels the request context but cannot stop a handler that ignores it.
- The Logger includes query strings by default. Turn that off with `WithIncludeQuery(false)` if anything secret travels in a URL.
