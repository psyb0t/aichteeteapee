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
- Caches 2xx responses only, skips errors
- Cache key = `sha256(method + url + headers + body)` with configurable header exclusions
- Custom cache key function via `CacheKeyFn`

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
