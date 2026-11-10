---
title: "Proxy"
description: "The prawxxey package: forward a request upstream, strip hop-by-hop headers, cap response size and cache public GET and HEAD answers."
nav_order: 5
---

# Proxy

Pronounced "proxy", because of course it is.

[`serbewr/prawxxey`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/prawxxey) forwards one request to an upstream server and hands back the status, headers and body, with optional response caching.

## Usage

```go
body, err := io.ReadAll(r.Body)
if err != nil {
    prawxxey.WriteError(w, http.StatusBadRequest)

    return
}

payload := &prawxxey.RequestPayload{
    Method:   r.Method,
    URL:      "https://api.example.com/data",
    Headers:  r.Header,
    Body:     body,
    ClientIP: aichteeteapee.GetClientIP(r),
    Proto:    prawxxey.RequestScheme(r),
}

result, err := prawxxey.ForwardRequest(r.Context(), prawxxey.ForwardConfig{
    HTTPClient:          &http.Client{Timeout: 30 * time.Second},
    Cache:               myCache, // nil means no caching
    CacheTTL:            5 * time.Minute,
    MaxResponseBodySize: 50 << 20, // 50MB, default 100MB
    CacheKeyExcludeHeaders: map[string]struct{}{
        http.CanonicalHeaderKey(aichteeteapee.HeaderNameXRequestID): {},
    },
}, payload)
if err != nil {
    prawxxey.WriteError(w, http.StatusBadGateway)

    return
}
```

`result` is a `*ResponseResult` with `StatusCode`, `Headers` and `Body`. Copying it onto your `http.ResponseWriter` is up to you, so you decide which upstream headers your clients see.

`RequestScheme(r)` returns `https` for a TLS request, else the `X-Forwarded-Proto` header, else `http`.

## ForwardConfig

| Field | Default | What it does |
| --- | --- | --- |
| `HTTPClient` | required | The client that sends the request. A nil client is an error. |
| `Cache` | nil | A [`common-go/cache.Cache`](https://github.com/psyb0t/common-go). Nil turns caching off. |
| `CacheTTL` | 5 minutes | How long a cached response lives. Zero means the default. |
| `CacheKeyFn` | `payload.HashExcluding(CacheKeyExcludeHeaders)` | Your own cache key. |
| `CacheKeyExcludeHeaders` | none | Headers left out of the default cache key, such as a per-request ID. |
| `MaxResponseBodySize` | 100MB | Upstream bodies bigger than this fail instead of filling your memory. Zero means the default; a negative value is an error. |

## What it does to the request

- Copies the payload headers, minus the hop-by-hop ones (`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `Te`, `Trailer`, `Trailers`, `Transfer-Encoding`, `Upgrade`) and any header named in `Connection`.
- Sets `X-Forwarded-For` and `X-Real-IP` to `ClientIP`, and `X-Forwarded-Proto` to `Proto`, when those are set. Both replace whatever the client sent.
- Refuses any `URL` that is not an absolute `http` or `https` URL with a host and no embedded credentials.

Treat `payload.URL` as trusted configuration. If it comes from a request, apply your own SSRF checks before calling `ForwardRequest`; this package does not block private addresses.

## Caching

With a `Cache` set, a response is served from or stored in the cache only when every one of these holds:

- the method is `GET` or `HEAD`;
- the `HTTPClient` has no cookie jar, because the jar can add session cookies after `ForwardRequest` sees the payload, and sharing that response would be a nasty little data leak;
- the request has no `Authorization` or `Cookie` header, no `Cache-Control: no-cache` or `no-store`, and no `Pragma: no-cache`;
- the response status is 2xx;
- the response has no `Set-Cookie` or `Vary` header and no `Cache-Control: private`, `no-cache` or `no-store`.

The response to a cacheable request gets an `X-Cache-Status` header of `HIT` or `MISS`. A cache read or decode failure sends the request upstream, and a cache write failure still returns the upstream response. Both are logged as warnings.

## Request fingerprinting

```go
hash := payload.Hash()
hash := payload.HashExcluding(map[string]struct{}{
    http.CanonicalHeaderKey(aichteeteapee.HeaderNameXRequestID): {},
})
```

A hex SHA-256 of the method, URL, sorted headers and body. Header names in the exclude map must match the payload's header keys byte for byte. Headers copied from an `http.Header` use the canonical form (`X-Request-Id`), while `aichteeteapee.HeaderNameXRequestID` is `X-Request-ID`, so pass names through `http.CanonicalHeaderKey` as above or the header is not excluded. The same goes for `CacheKeyExcludeHeaders`.

## Errors

`ForwardRequest` returns an error for a nil context, payload or client, an invalid URL or size limit, a transport failure and an oversized body. The error never contains the upstream URL, and the logs only carry the method, status, duration and size.

```go
prawxxey.WriteError(w, http.StatusBadGateway)
```

`WriteError` writes the standard [error envelope](errors.md) with the code that `ErrorCodeFromHTTPStatus` maps the status to and the standard status text as the message.
