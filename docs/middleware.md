---
title: "Middleware"
description: "Every serbewr/middleware option and default: RequestID, Logger, Recovery, BasicAuth, BearerAuth, CORS, SecurityHeaders, Timeout, content type checks."
nav_order: 3
---

# Middleware

[`serbewr/middleware`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/middleware) is the whole stack, and the pieces play nice together. A `Middleware` is a plain `func(http.Handler) http.Handler`, so anything from the wider Go ecosystem slots in next to these.

```go
handler := middleware.Chain(finalHandler,
    middleware.RequestID(), // outermost: runs first on the way in, last on the way out
    middleware.Logger(),
    middleware.Recovery(),
)
```

`Chain` wraps right to left, so the first middleware in the list sees the request first. The [server](server.md#router) uses `Chain` for global, group and route middleware.

Order matters in two places. Put `RequestID` before `Logger`, or the request ID never reaches the log line. Put `CORS` before anything that rejects requests (auth, content type checks), so browser preflights get their answer.

Every middleware logs through `ctxscope.GetLogger(ctx)`. [Logging](logging.md) explains how the request fields get onto every line.

## RequestID

```go
middleware.RequestID()
```

Takes the incoming `X-Request-ID` if it is at most 128 characters of `[a-zA-Z0-9._-]`, otherwise generates a UUID4. It sets the ID on the response header, on the request context (read it with `aichteeteapee.GetRequestID(r)`), and on the log scope as `requestId`. The [client](client.md) forwards it on outbound calls made with that context.

## Logger

One log line per request, written after the handler returns.

```go
middleware.Logger(
    middleware.WithLogLevel(slog.LevelDebug),
    middleware.WithLogMessage("request done"),
    middleware.WithSkipPaths("/health", "/metrics"),
    middleware.WithExtraFields(map[string]any{"service": "api"}),
    middleware.WithIncludeQuery(false),
    middleware.WithIncludeHeaders(aichteeteapee.HeaderNameXCorrelationID),
)
```

| Option | Default | Effect |
| --- | --- | --- |
| `WithLogLevel` | `slog.LevelInfo` | Level of the request line. |
| `WithLogMessage` | `HTTP request` | Message of the request line. |
| `WithSkipPaths` | none | Paths that are not logged. Matched after `path.Clean`, so `/health/` and `/health` both match. |
| `WithExtraFields` | none | Fixed fields added to every line. |
| `WithIncludeQuery` | `true` | Adds the raw query string as `query`. Turn it off if tokens ever travel in query strings. |
| `WithIncludeHeaders` | none | Adds each named request header as `header_<Name>` when present. |

Each line carries `method`, `path`, `ip` (from `X-Forwarded-For`, then `X-Real-IP`, then the remote address), `status`, `duration` and `userAgent`. Logger also puts `method`, `path` and `ip` on the scope, so handlers downstream log them too.

## Recovery

Catches a panic, logs it, and answers with an error response instead of killing the connection.

```go
middleware.Recovery(
    middleware.WithRecoveryStatusCode(http.StatusInternalServerError),
    middleware.WithRecoveryResponse(customErrorBody),
    middleware.WithRecoveryContentType(aichteeteapee.ContentTypeJSON),
    middleware.WithIncludeStack(false),
    middleware.WithRecoveryLogLevel(slog.LevelWarn),
    middleware.WithRecoveryLogMessage("handler blew up"),
    middleware.WithRecoveryExtraFields(map[string]any{"component": "api"}),
)
```

By default it logs at error level with the panic value as `error` and the stack trace as `stack`, then answers `500` with `ErrorResponseInternalServerError` as JSON. With a non-JSON content type, the response body is written only if it is a `string`.

`WithCustomRecoveryHandler(func(recovered any, w http.ResponseWriter, r *http.Request))` replaces all of that, logging included.

## BasicAuth

HTTP Basic authentication.

```go
middleware.BasicAuth(
    middleware.WithBasicAuthUsers(map[string]string{
        "admin": "secret",
    }),
    middleware.WithBasicAuthRealm("admin-area"),
    middleware.WithBasicAuthSkipPaths("/health", "/metrics"),
    middleware.WithBasicAuthChallenge(true),
    middleware.WithBasicAuthUnauthorizedMessage("nope"),
)
```

| Option | Default | Effect |
| --- | --- | --- |
| `WithBasicAuthUsers` | none | User name to password map. Repeated calls merge. |
| `WithBasicAuthValidator` | none | `func(user, pass string) bool` that replaces the user map entirely. |
| `WithBasicAuthRealm` | `restricted` | Realm in `WWW-Authenticate`. |
| `WithBasicAuthChallenge` | `true` | Send `WWW-Authenticate`, which makes browsers show the login prompt. |
| `WithBasicAuthUnauthorizedMessage` | `Unauthorized` | `message` in the 401 body. |
| `WithBasicAuthSkipPaths` | none | Paths that skip auth, matched after `path.Clean`. |
| `WithConstantTimeComparison` | `true` | Compare SHA-256 hashes of the passwords in constant time, so neither timing nor length leaks. |

A failure answers `401` with the JSON envelope (`code` `UNAUTHORIZED`). With no users and no validator, every request fails. On success the user name goes on the request context under `aichteeteapee.ContextKeyUser`, which [EchoHandler](server.md#echohandler) shows.

## BearerAuth

`Authorization: Bearer <token>` for APIs, the companion to `BasicAuth` for the usual "API key from an env var" case.

```go
middleware.BearerAuth(
    middleware.WithBearerAuthTokens(cfg.APIToken), // one or more accepted tokens
    middleware.WithBearerAuthSkipPaths("/health", "/metrics"),
    middleware.WithBearerAuthUnauthorizedMessage("nope"),
)
```

| Option | Default | Effect |
| --- | --- | --- |
| `WithBearerAuthTokens` | none | Accepted tokens. Empty strings are ignored, so an unset env var never becomes a valid token. |
| `WithBearerAuthValidator` | none | `func(token string) bool` that replaces the token set entirely. |
| `WithBearerAuthSkipPaths` | none | Paths that skip auth, matched after `path.Clean`. |
| `WithBearerAuthUnauthorizedMessage` | `Unauthorized` | `message` in the 401 body. |
| `WithBearerAuthConstantTimeComparison` | `true` | Compare SHA-256 hashes against every token without an early exit. |

The `Bearer` scheme is matched case-insensitively. A failure answers `401` with the JSON envelope and no `WWW-Authenticate` header. With no tokens and no validator, every request fails.

## CORS

Blocks unknown origins by default, because that's what a sane default looks like. Use `WithAllowAllOrigins()` or `aichteeteapee.FuckSecurity()` when you need to go permissive.

```go
middleware.CORS(
    middleware.WithAllowedOrigins("https://app.example.com", "https://admin.example.com"),
    middleware.WithAllowedMethods(http.MethodGet, http.MethodPost),
    middleware.WithAllowedHeaders(
        aichteeteapee.HeaderNameContentType,
        aichteeteapee.HeaderNameAuthorization,
    ),
    middleware.WithExposedHeaders("X-Total-Count"),
    middleware.WithMaxAge(3600),
    middleware.WithAllowCredentials(true),
)
```

| Option | Default |
| --- | --- |
| `WithAllowedOrigins` | none, so no origin is allowed |
| `WithAllowAllOrigins` | off, unless `FuckSecurity()` was called before `CORS()` |
| `WithAllowedMethods` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `WithAllowedHeaders` | `Authorization, Content-Type, X-Request-ID` |
| `WithExposedHeaders` | none |
| `WithMaxAge` | `86400` seconds |
| `WithAllowCredentials` | `false` |

An allowed origin is echoed back in `Access-Control-Allow-Origin` together with `Vary: Origin`, so caches keep per-origin copies. With all origins allowed it sends `*`; browsers refuse `*` on credentialed requests, so list the origins if you need cookies. The methods, headers and max age go out on every response. Every `OPTIONS` request gets `204 No Content` and never reaches your handler.

## SecurityHeaders

Adds the usual security headers. Each one can be changed or turned off.

```go
middleware.SecurityHeaders(
    middleware.WithContentSecurityPolicy("default-src 'self'"),
    middleware.DisableHSTS(),
)
```

| Header | Default | Change | Turn off |
| --- | --- | --- | --- |
| `X-Content-Type-Options` | `nosniff` | `WithXContentTypeOptions` | `DisableXContentTypeOptions` |
| `X-Frame-Options` | `DENY` | `WithXFrameOptions` | `DisableXFrameOptions` |
| `X-XSS-Protection` | `1; mode=block` | `WithXXSSProtection` | `DisableXXSSProtection` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | `WithStrictTransportSecurity` | `DisableHSTS` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | `WithReferrerPolicy` | `DisableReferrerPolicy` |
| `Content-Security-Policy` | not sent | `WithContentSecurityPolicy` | `DisableCSP` |

## Timeout

Gives each request a deadline and answers `504 Gateway Timeout` when it passes.

```go
middleware.Timeout(middleware.WithTimeout(15 * time.Second))

// Presets
middleware.Timeout(middleware.WithShortTimeout())   // 5s
middleware.Timeout(middleware.WithDefaultTimeout()) // 10s, also what Timeout() uses
middleware.Timeout(middleware.WithLongTimeout())    // 30s
```

The handler runs in its own goroutine with a context that is cancelled at the deadline. If the handler has not written anything by then, the client gets `504` with code `GATEWAY_TIMEOUT`. If it already started writing, the client keeps what was written. Either way, every write after the deadline fails with `http.ErrHandlerTimeout`.

Handlers must respect `ctx.Done()`. The timeout cancels the context but can't kill a goroutine that ignores it. If your handler doesn't check the context, that's on you.

## EnforceRequestContentType

Rejects requests whose `Content-Type` is not in the list. `GET`, `HEAD` and `DELETE` pass without a check.

```go
middleware.EnforceRequestContentType(
    aichteeteapee.ContentTypeJSON,
    aichteeteapee.ContentTypeXML,
)

// Convenience
middleware.EnforceRequestContentTypeJSON()
```

Parameters such as `; charset=utf-8` are ignored and the match is case-insensitive. A missing header answers `400 MISSING_CONTENT_TYPE`. A wrong one answers `415 UNSUPPORTED_CONTENT_TYPE` with `details` listing what was received and what is allowed. `OPTIONS` is checked too, so put `CORS` in front of it.

## Helpers

- `middleware.GetRequestID(r)` and `middleware.GetClientIP(r)` behave like the root package's `aichteeteapee.GetRequestID` and `aichteeteapee.GetClientIP`.
- `middleware.BaseResponseWriter` is the embeddable base for your own response writer wrappers. It passes `Hijack`, `Flush` and `Unwrap` through to the real writer, so WebSocket upgrades and streamed responses keep working behind your wrapper. Logger and Timeout use it.
