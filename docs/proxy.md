# serbewr/prawxxey — HTTP request forwarding

Pronounced "proxy", because of course it is.

Forward requests to upstream servers with optional response caching.

## Usage

```go
result, err := prawxxey.ForwardRequest(ctx, prawxxey.ForwardConfig{
    HTTPClient:          http.DefaultClient,
    Cache:               myCache,          // nil = no caching
    CacheTTL:            5 * time.Minute,
    MaxResponseBodySize: 50 << 20,         // 50MB, default 100MB
    CacheKeyExcludeHeaders: map[string]struct{}{
        aichteeteapee.HeaderNameXRequestID: {},
    },
}, payload)
```

`result` contains `StatusCode`, `Headers`, and `Body`.

## Request payload

```go
payload := &prawxxey.RequestPayload{
    Method:   http.MethodGet,
    URL:      "https://api.example.com/data",
    Headers:  r.Header,
    Body:     requestBody,
    ClientIP: aichteeteapee.GetClientIP(r),
    Proto:    prawxxey.RequestScheme(r),
}
```

## What it does

- Sets `X-Forwarded-For`, `X-Real-IP`, `X-Forwarded-Proto` on upstream requests — the stuff proxies are supposed to do
- Strips hop-by-hop headers per RFC 2616
- Limits response body size (default 100MB, configurable) — so a rogue upstream can't OOM you
- Caches public 2xx `GET` and `HEAD` responses only
- Skips the cache for requests with `Authorization`, `Cookie`,
  `Cache-Control: no-cache`, `Cache-Control: no-store`, or
  `Pragma: no-cache`
- Skips the cache when the configured HTTP client has a cookie jar. The jar can
  add session cookies after `ForwardRequest` sees the payload, and sharing that
  response would be a nasty little data leak
- Skips response caching for `Set-Cookie`, `Vary`, `Cache-Control: private`,
  `Cache-Control: no-cache`, and `Cache-Control: no-store`
- Cache key = `sha256(method + url + headers + body)` with configurable header exclusions
- Custom cache key function via `CacheKeyFn`

Upstream URLs must be absolute HTTP or HTTPS URLs with a host and no embedded
credentials. Treat `payload.URL` as trusted configuration. If it comes from a
request, apply the service's SSRF checks before calling `ForwardRequest`.

## Request fingerprinting

```go
hash := payload.Hash()
hash := payload.HashExcluding(map[string]struct{}{
    aichteeteapee.HeaderNameXRequestID: {},
})
```

Deterministic SHA-256 hash of method + URL + sorted headers + body.

## Error handling

```go
prawxxey.WriteError(w, http.StatusBadGateway)
```

Writes a standard `aichteeteapee.ErrorResponse` JSON body with the appropriate error code.
