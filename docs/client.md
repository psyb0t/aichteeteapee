# HTTP client

Pronounced "client". You get it. This is the outbound half of
aichteeteapee. It uses the root package's header names, content types, request
ID, status sentinels, and `ErrorResponse` contract.

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

`Get`, `Head`, `Options`, `Delete`, `Post`, `Put`, and `Patch` call `Do`.
Every 2xx status is successful. Empty success bodies do not require a response
target.

Structured request values are encoded as JSON. `[]byte`, `string`,
`io.Reader`, and `url.Values` send binary, text, reader-provided, and form
bodies. Responses decode as JSON by default. A `*[]byte` or `io.Writer` target
receives the bounded raw response body.

## Errors and limits

Response bodies are limited to 10 MiB by default. Change the limit with
`WithMaxResponseBodySize`. A body above the limit matches
`ErrResponseBodyTooLarge` through `errors.Is`.

A non-2xx response returns both `*Response` and `*HTTPError`. The error keeps
the decoded `aichteeteapee.ErrorResponse` and unwraps to the matching root
status sentinel when one exists.

```go
var httpError *klhayyent.HTTPError
if errors.As(err, &httpError) {
    log.Error("dependency rejected request", "code", httpError.ErrorResponse.Code)
}
```

Transport errors, timeouts, and context cancellation keep their original error
chain.

## Cookies

Cookie handling is on by default. Each client gets an RFC 6265 cookie jar
backed by the public suffix list. Cookies set by one response are scoped and
sent on later requests. The standard library jar handles `Secure`, domain,
path, expiry, and HttpOnly attributes.

Use `WithCookieJar` to share or replace the jar. `WithHTTPClient` preserves a
jar already installed on the supplied client unless `WithCookieJar` overrides
it. Construction shallow-copies the supplied `http.Client`, so timeout,
redirect, and jar changes made by `klhayyent` do not mutate the caller's value.

## Security boundary

The base URL is trusted configuration. It must be an absolute HTTP or HTTPS URL
without user information, a query, or a fragment. Request targets must be
relative. Absolute targets, scheme-relative targets, control characters,
backslashes, fragments, and encoded traversal segments are rejected before the
transport runs.

Redirects stay on the base URL's scheme and host and stop after ten hops.
`WithCheckRedirect` may impose a stricter policy, but cannot bypass the
same-origin check. This prevents redirect-based credential and cookie
disclosure. The client never logs URLs, queries, headers, cookies, or bodies.

For services that must accept an untrusted destination, validate it before
constructing a client and apply the service's SSRF policy, including private
address and DNS rebinding controls. `klhayyent` intentionally treats the base
URL as trusted dependency configuration.

## Configuration

- `WithHTTPClient` copies an existing `http.Client`.
- `WithTransport` supplies a transport.
- `WithTimeout` changes the total request timeout.
- `WithCookieJar` supplies a cookie jar.
- `WithDefaultHeader` and `WithDefaultHeaders` set immutable defaults.
- `WithMaxResponseBodySize` changes the buffered response limit.
- `WithDependencyName` sets the bounded dependency label used in logs.
- `WithCheckRedirect` adds a stricter redirect callback.

Request options include `WithHeader`, `WithHeaders`, `WithBearerToken`,
`WithBasicAuth`, `WithQuery`, and `WithQueryValues`. Request options operate on
per-call copies and are safe to use with concurrent requests.
