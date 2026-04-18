# serbewr/middleware — middleware stack

Every middleware uses `slogging.GetLogger(ctx)` for structured logging with context propagation. The chain builds up the logger progressively — RequestID adds the request ID, Logger adds method/path/ip, and any downstream code gets all fields automatically via `slogging.GetLogger(ctx)`.

## RequestID

Generates a UUID4 or extracts an existing `X-Request-ID` from the request. Validated: max 128 chars, `[a-zA-Z0-9._-]` only — malformed IDs are replaced with a fresh UUID. Sets it on the context logger and response header.

```go
middleware.RequestID()
```

## Logger

Structured request/response logging.

```go
middleware.Logger(
    middleware.WithLogLevel(slog.LevelDebug),
    middleware.WithSkipPaths("/health", "/metrics"),
    middleware.WithExtraFields(map[string]any{"service": "api"}),
    middleware.WithIncludeQuery(true),
    middleware.WithIncludeHeaders(
        aichteeteapee.HeaderNameAuthorization,
    ),
)
```

Logs method, path, client IP, status code, duration, and user agent. Skip paths use `path.Clean` normalization — `/health/` and `/health` both match.

## Recovery

Catches panics, logs with optional stack trace, returns a configurable error response.

```go
middleware.Recovery(
    middleware.WithRecoveryStatusCode(http.StatusInternalServerError),
    middleware.WithRecoveryResponse(customErrorBody),
    middleware.WithRecoveryContentType(aichteeteapee.ContentTypeJSON),
    middleware.WithIncludeStack(true),
    middleware.WithCustomRecoveryHandler(func(recovered any, w http.ResponseWriter, r *http.Request) {
        // full control over panic response
    }),
)
```

## BasicAuth

HTTP Basic Authentication with constant-time comparison (SHA-256 normalized to prevent length-based timing leaks).

```go
middleware.BasicAuth(
    middleware.WithBasicAuthUsers(map[string]string{
        "admin": "secret",
    }),
    middleware.WithBasicAuthRealm("admin-area"),
    middleware.WithBasicAuthSkipPaths("/health", "/metrics"),
    middleware.WithBasicAuthChallenge(true),
    middleware.WithBasicAuthValidator(func(user, pass string) bool {
        return myCustomAuth(user, pass)
    }),
)
```

Skip paths use `path.Clean` normalization. Custom validators bypass the built-in user map entirely.

## CORS

Blocks unknown origins by default. Use `WithAllowAllOrigins()` or `aichteeteapee.FuckSecurity()` to go permissive.

```go
middleware.CORS(
    middleware.WithAllowedOrigins("https://app.example.com", "https://admin.example.com"),
    middleware.WithAllowedMethods(http.MethodGet, http.MethodPost),
    middleware.WithAllowedHeaders(
        aichteeteapee.HeaderNameContentType,
        aichteeteapee.HeaderNameAuthorization,
    ),
    middleware.WithExposedHeaders("X-Total-Count"),
    middleware.WithMaxAge(86400),
    middleware.WithAllowCredentials(true),
)
```

Preflight `OPTIONS` requests return `204 No Content`. When specific origins are configured, `Vary: Origin` is set for CDN cache correctness.

## SecurityHeaders

Adds standard security headers with sane defaults. Each can be customized or disabled individually.

```go
middleware.SecurityHeaders(
    middleware.WithContentSecurityPolicy("default-src 'self'"),
    middleware.DisableHSTS(),
)
```

| Header | Default |
|---|---|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Content-Security-Policy` | _(empty — not set unless configured)_ |

## Timeout

Request deadline with `504 Gateway Timeout` response on expiry.

```go
middleware.Timeout(
    middleware.WithTimeout(15 * time.Second),
)

// Presets
middleware.Timeout(middleware.WithShortTimeout())  // 5s
middleware.Timeout(middleware.WithDefaultTimeout()) // 10s
middleware.Timeout(middleware.WithLongTimeout())    // 30s
```

Handlers must respect `ctx.Done()` — the timeout cancels the context but cannot kill a goroutine that ignores it.

## EnforceRequestContentType

Rejects requests with wrong `Content-Type`. Skips `GET`, `HEAD`, and `DELETE`.

```go
middleware.EnforceRequestContentType(
    aichteeteapee.ContentTypeJSON,
    aichteeteapee.ContentTypeXML,
)

// Convenience
middleware.EnforceRequestContentTypeJSON()
```
