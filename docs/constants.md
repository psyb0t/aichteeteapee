---
title: "Constants and helpers"
description: "The root aichteeteapee package: header names, content types, schemes, context keys, log fields, every Default constant and the request helpers."
nav_order: 8
---

# Constants and helpers

Everything you need to stop hardcoding strings in your HTTP code. The root package, `github.com/psyb0t/aichteeteapee`, holds the constants and small helpers every other package uses. Error codes and envelopes have their own page, [Errors](errors.md).

## Request and response helpers

| Function | What it does |
| --- | --- |
| `WriteJSON(w, status, data)` | Sets `Content-Type: application/json`, writes the status, encodes `data` indented by two spaces. |
| `GetClientIP(r)` | The first address in `X-Forwarded-For`, else `X-Real-IP`, else the host part of `r.RemoteAddr`. |
| `GetRequestID(r)` | The ID the [RequestID middleware](middleware.md#requestid) stored on the context, or `""`. |
| `IsRequestContentType(r, ct)` | Whether the request's media type equals `ct`, ignoring parameters and case. |
| `IsRequestContentTypeJSON(r)`, `...XML(r)`, `...ApplicationFormURLEncoded(r)`, `...MultipartFormData(r)` | Shortcuts for the common ones. |

`GetClientIP` believes the forwarding headers, and anyone can send those. Only use its answer for anything that matters (rate limits, allow lists) when a proxy you control sets or overwrites them.

## Content types

[`content_types.go`](https://github.com/psyb0t/aichteeteapee/blob/main/content_types.go):

| Constant | Value |
| --- | --- |
| `ContentTypeJSON` | `application/json` |
| `ContentTypeYAML` | `application/yaml` |
| `ContentTypeXYAML` | `application/x-yaml` (legacy, prefer `ContentTypeYAML`) |
| `ContentTypeXML` | `application/xml` |
| `ContentTypeTextPlain` | `text/plain` |
| `ContentTypeHTML` | `text/html` |
| `ContentTypeHTMLUTF8` | `text/html; charset=UTF-8` |
| `ContentTypeTextEventStream` | `text/event-stream` |
| `ContentTypeOctetStream` | `application/octet-stream` |
| `ContentTypeImageJPG` | `image/jpeg` |
| `ContentTypeImagePNG` | `image/png` |
| `ContentTypeMultipartFormData` | `multipart/form-data` |
| `ContentTypeApplicationFormURLEncoded` | `application/x-www-form-urlencoded` |

## Header names

[`headers.go`](https://github.com/psyb0t/aichteeteapee/blob/main/headers.go) has a `HeaderName*` constant for every header you'll ever need, grouped by job: authentication (`HeaderNameAuthorization`, `HeaderNameXAPIKey`, `HeaderNameWWWAuthenticate`), cookies, content negotiation, request tracking (`HeaderNameXRequestID`, `HeaderNameXCorrelationID`), client info (`HeaderNameXForwardedFor`, `HeaderNameXRealIP`, `HeaderNameXClientID` and friends), CORS, cache control, hop-by-hop, security, rate limiting, response metadata, WebSocket handshake, and a few strays.

The same file has the auth scheme prefixes, `AuthSchemeBearer` (`"Bearer "`) and `AuthSchemeBasic` (`"Basic "`), trailing space included, and the `X-Cache-Status` values the [proxy](proxy.md#caching) sets, `CacheStatusHit` and `CacheStatusMiss`.

The constants use the conventional spelling (`X-Request-ID`), which is not always Go's canonical form (`X-Request-Id`). `http.Header.Get` and `Set` canonicalize for you; a raw map lookup does not.

## Schemes and networks

[`network_types.go`](https://github.com/psyb0t/aichteeteapee/blob/main/network_types.go): `SchemeHTTP`, `SchemeHTTPS`, and the `net.Listen` network names `NetworkTypeTCP`, `NetworkTypeTCP4`, `NetworkTypeTCP6`, `NetworkTypeUDP`, `NetworkTypeUDP4`, `NetworkTypeUDP6`, `NetworkTypeUnix`, `NetworkTypeUnixgram`, `NetworkTypeUnixpacket`.

## Context keys

`ContextKey` is a string type, so these never collide with plain string keys:

| Constant | Set by |
| --- | --- |
| `ContextKeyRequestID` | RequestID middleware |
| `ContextKeyUser` | BasicAuth middleware, after a successful login |

## Log field names

[`log_fields.go`](https://github.com/psyb0t/aichteeteapee/blob/main/log_fields.go) names the structured log keys the library writes: `FieldRequestID` (`requestId`), `FieldMethod`, `FieldPath`, `FieldIP`, `FieldStatus`, `FieldDuration`, `FieldQuery`, `FieldUserAgent` for requests, plus WebSocket keys such as `FieldClientID`, `FieldConnectionID`, `FieldHubName`, `FieldEventType` and `FieldEventID`. Use them in your own logs and your lines line up with the library's. See [Logging](logging.md).

## Defaults

[`defaults.go`](https://github.com/psyb0t/aichteeteapee/blob/main/defaults.go) holds every default value as a `Default*` constant, so you can read what the library will do without guessing:

| Area | Constants |
| --- | --- |
| Server | `DefaultHTTPServerListenAddress` (`127.0.0.1:8080`), `...ReadTimeout` (15s), `...ReadHeaderTimeout` (10s), `...WriteTimeout` (30s), `...IdleTimeout` (60s), `...MaxHeaderBytes` (1MB), `...ShutdownTimeout` (10s), `...ServiceName` (`http-server`) |
| TLS | `DefaultHTTPServerTLSEnabled` (false), `...TLSListenAddress` (`127.0.0.1:8443`), `...TLSCertFile`, `...TLSKeyFile` (empty) |
| Uploads | `DefaultFileUploadMaxMemory` (32MB), `DefaultFileUploadMaxSize` (100MB) |
| Timeouts | `DefaultHTTPRequestTimeout` (30s), `DefaultHTTPClientTimeout` (30s, used by the client) |
| CORS | `DefaultCORSAllowOriginAll` (`*`), `DefaultCORSMaxAge` (86400), plus `GetDefaultCORSAllowMethods()` and `GetDefaultCORSAllowHeaders()` |
| Security headers | `DefaultSecurityXContentTypeOptionsNoSniff`, `...XFrameOptionsDeny`, `...XXSSProtectionBlock`, `...StrictTransportSecurity`, `...ReferrerPolicyStrictOrigin` |
| Auth | `DefaultBasicRealmName` (`restricted`), `DefaultUnauthorizedMessage` (`Unauthorized`) |
| WebSocket clients | `DefaultWebSocketClientSendBufferSize` (256), `...ReadBufferSize`, `...WriteBufferSize` (1024), `...ReadLimit` (1MB), `...ReadTimeout` (60s), `...WriteTimeout` (10s), `...PingInterval` (54s), `...PongTimeout` (60s) |
| WebSocket upgrades | `DefaultWebSocketHandlerReadBufferSize`, `...WriteBufferSize` (1024), `...HandshakeTimeout` (45s), `...EnableCompression` (false) |
| Paths | `DefaultAPIRootPath` (`/api`), `StandardAPIOASPath` (`/openapi.yaml`), `StandardAPISwaggerUIPath` (`/docs/*`), `FileNameIndexHTML` (`index.html`) |

The same file has the security switches, `FuckSecurity()`, `UnfuckSecurity()` and `IsDevMode()`, and the origin checks built on them. [Security](security.md) covers those.

The `EnvVarName*` constants in [`env_var_names.go`](https://github.com/psyb0t/aichteeteapee/blob/main/env_var_names.go) are the server's env var names; the [server configuration table](server.md#configuration) lists them with their defaults.
