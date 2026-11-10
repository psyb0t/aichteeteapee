---
title: "Client"
description: "The klhayyent HTTP client: method helpers, body encoding, bounded responses, HTTPError and status sentinels, cookies, redirects and options."
nav_order: 6
---

# Client

Pronounced "client". Apparently that one needed saying too.

[`klhayyent`](https://github.com/psyb0t/aichteeteapee/tree/main/klhayyent) is the outbound half of aichteeteapee. It handles outbound HTTP without making every service rebuild the same wrapper, and it speaks the root package's header names, content types, request ID and [`ErrorResponse`](errors.md) envelope. One `Client` is safe to use from many goroutines.

## Basic use

```go
client, err := klhayyent.New("https://api.example.com/v1")
if err != nil {
    return err
}

var output Widget

response, err := client.Get(
    ctx,
    "/widgets/42",
    &output,
    klhayyent.WithBearerToken(token),
    klhayyent.WithQuery("view", "summary"),
)
if err != nil {
    return err
}
```

`Get`, `Options` and `Delete` take a response target; `Post`, `Put` and `Patch` take a request body and a response target; `Head` takes neither. They all call `Do(ctx, method, target, requestBody, responseBody, options...)`. Every 2xx status is a success, and an empty success body needs no target.

Targets are joined onto the base URL's path, so `/widgets/42` on `https://api.example.com/v1` requests `https://api.example.com/v1/widgets/42`. The returned `*Response` holds `StatusCode`, `Header` and the buffered `Body`.

## Bodies

The request body is encoded by its Go type, and `Content-Type` is set unless you set it yourself:

| Request body | Sent as | `Content-Type` |
| --- | --- | --- |
| `nil` | no body | none |
| `io.Reader` | streamed as is | none, set it with `WithHeader` |
| `[]byte` | raw bytes | `application/octet-stream` |
| `string` | text | `text/plain` |
| `url.Values` | form encoded | `application/x-www-form-urlencoded` |
| anything else | `json.Marshal` | `application/json` |

The response decodes as JSON into the target by default. A `*[]byte` target gets the raw body, and an `io.Writer` target has the raw body written to it.

Every request sends `Accept: application/json` unless you change the default headers. When the context carries a request ID from the [RequestID middleware](middleware.md#requestid), the client sends it as `X-Request-ID`, so one ID follows a request across services.

## Errors and limits

Response bodies are limited to 10 MiB by default (`DefaultMaxResponseBodySize`). Change the limit with `WithMaxResponseBodySize`. A bigger body fails with an error that matches `ErrResponseBodyTooLarge` through `errors.Is`.

A non-2xx response returns both the `*Response` and an `*HTTPError`. The error holds the decoded `aichteeteapee.ErrorResponse`. When the body is not that envelope, the client fills one in from the status: the code from `ErrorCodeFromHTTPStatus` and the standard status text as the message.

```go
var httpError *klhayyent.HTTPError
if errors.As(err, &httpError) {
    log.Error("dependency rejected request", "code", httpError.ErrorResponse.Code)
}

if errors.Is(err, aichteeteapee.ErrNotFound) {
    // 404
}
```

`HTTPError` unwraps to the root package sentinel for its status:

| Status | Sentinel |
| --- | --- |
| 400 | `ErrBadRequest` |
| 401 | `ErrUnauthorized` |
| 403 | `ErrForbidden` |
| 404 | `ErrNotFound` |
| 405 | `ErrMethodNotAllowed` |
| 409 | `ErrConflict` |
| 410 | `ErrGone` |
| 422 | `ErrUnprocessableEntity` |
| 429 | `ErrTooManyRequests` |
| 500 | `ErrInternalServer` |
| 502 | `ErrBadGateway` |
| 503 | `ErrServiceUnavailable` |
| 504 | `ErrGatewayTimeout` |
| anything else | `ErrUnexpectedResponseStatus` |

Timeouts and context cancellation keep their error chain, so `errors.Is(err, context.DeadlineExceeded)` works. The `*url.Error` layer is stripped first, so the URL never ends up in an error message or a log. Construction and option mistakes match `ErrInvalidBaseURL` or `ErrInvalidOption`, and a rejected target matches `ErrInvalidRequestTarget`.

## Cookies

Cookie handling is on by default. Each client gets an RFC 6265 cookie jar backed by the public suffix list, so cookies set by one response are scoped and sent on later requests. The standard library jar handles `Secure`, domain, path, expiry and `HttpOnly`.

Use `WithCookieJar` to share or replace the jar, and `client.CookieJar()` to read it. `WithHTTPClient` keeps a jar already installed on the supplied client unless `WithCookieJar` overrides it. Construction shallow-copies the supplied `http.Client`, so timeout, redirect and jar changes made by `klhayyent` do not mutate the caller's value.

## Security boundary

The base URL is trusted configuration. It must be an absolute HTTP or HTTPS URL without user information, a query or a fragment. Request targets must be relative. Absolute targets, scheme-relative targets, control characters, backslashes, fragments and encoded traversal segments are rejected before the transport runs.

Redirects stay on the base URL's scheme and host and stop after ten hops. `WithCheckRedirect` can add a stricter policy but cannot bypass the same-origin check. That blocks redirect-based credential and cookie disclosure. The client never logs URLs, queries, headers, cookies or bodies; a request log line has the dependency name, method, status, duration and outcome.

For services that must accept an untrusted destination, validate it before constructing a client and apply the service's SSRF policy, including private address and DNS rebinding controls. `klhayyent` intentionally treats the base URL as trusted dependency configuration.

## Client options

| Option | Default | Effect |
| --- | --- | --- |
| `WithHTTPClient` | a new `http.Client` | Use a shallow copy of an existing client. |
| `WithTransport` | `http.DefaultTransport` | Use this `http.RoundTripper`. |
| `WithTimeout` | 30s | Total request timeout. Zero turns it off. |
| `WithCookieJar` | public suffix jar | Use this cookie jar. |
| `WithDefaultHeader` | `Accept: application/json` | Set one header on every request. |
| `WithDefaultHeaders` | | Replace all default headers. |
| `WithMaxResponseBodySize` | 10 MiB | Largest buffered response body. Must be positive. |
| `WithDependencyName` | `http_client` | Name in the request log lines. Keep it a short fixed label. |
| `WithCheckRedirect` | none | Extra redirect policy, run after the same-origin check. |

A nil, empty or negative value where one is required makes `New` fail with `ErrInvalidOption`.

## Request options

`WithHeader`, `WithHeaders`, `WithBearerToken`, `WithBasicAuth`, `WithQuery` and `WithQueryValues` apply to one call. They work on per-call copies, so they are safe with concurrent requests. Header names and values are validated, and query values are merged with any query already on the target.
