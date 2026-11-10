# aichteeteapee

[![Go Reference](https://pkg.go.dev/badge/github.com/psyb0t/aichteeteapee.svg)](https://pkg.go.dev/github.com/psyb0t/aichteeteapee)
[![CI](https://github.com/psyb0t/aichteeteapee/actions/workflows/pipeline.yml/badge.svg?branch=main)](https://github.com/psyb0t/aichteeteapee/actions/workflows/pipeline.yml)
[![coverage](https://raw.githubusercontent.com/psyb0t/aichteeteapee/badges/coverage.svg)](https://github.com/psyb0t/aichteeteapee/actions/workflows/pipeline.yml)
[![version](https://raw.githubusercontent.com/psyb0t/aichteeteapee/badges/version.svg)](https://github.com/psyb0t/aichteeteapee/tags)
[![license](https://raw.githubusercontent.com/psyb0t/aichteeteapee/badges/license.svg)](https://github.com/psyb0t/aichteeteapee/blob/main/LICENSE)
[![imported by](https://raw.githubusercontent.com/psyb0t/aichteeteapee/badges/importers.svg)](https://github.com/psyb0t/aichteeteapee/blob/badges/importers.md)

Pronounced "HTTP". The name is the whole joke. Moving on.

A Go HTTP library that does everything you need and nothing you don't. Spin up a server with middleware, WebSocket hubs, file uploads, static serving, and request proxying with caching. Secure defaults, without the boilerplate.

```bash
go get github.com/psyb0t/aichteeteapee
```

```go
srv, err := serbewr.New()
if err != nil {
    return err
}

router := &serbewr.Router{
    GlobalMiddlewares: []middleware.Middleware{
        middleware.RequestID(),
        middleware.Logger(),
        middleware.Recovery(),
        middleware.SecurityHeaders(),
        middleware.CORS(),
    },
    Groups: []serbewr.GroupConfig{{
        Path: "/",
        Routes: []serbewr.RouteConfig{{
            Method: http.MethodGet,
            Path:   "/hello",
            Handler: func(w http.ResponseWriter, _ *http.Request) {
                aichteeteapee.WriteJSON(w, http.StatusOK, map[string]string{"msg": "hi"})
            },
        }},
    }},
}

return srv.Start(ctx, router)
```

## What's in the box

Every package name is how you'd say it out loud.

- **`aichteeteapee`**, the root package. Everything you need to stop hardcoding strings in your HTTP code: header names, content types, error codes, the `ErrorResponse` envelope, sentinel errors, defaults, `GetClientIP`, `GetRequestID`, `WriteJSON`.
- **[`serbewr`](https://psyb0t.github.io/aichteeteapee/server/)**, pronounced "server". D'oooh you kno. `net/http` with Go 1.22+ `ServeMux` routing, grouped routes with per-group middleware, health, echo and upload handlers, static files with directory indexing, TLS, config from env vars or code.
- **[`serbewr/middleware`](https://psyb0t.github.io/aichteeteapee/middleware/)**: RequestID, Logger, Recovery, BasicAuth, BearerAuth, CORS, SecurityHeaders, Timeout, EnforceRequestContentType. Set them up once and every log line gets the full request context for free.
- **[`serbewr/prawxxey`](https://psyb0t.github.io/aichteeteapee/proxy/)**, pronounced "proxy", because of course it is. Forward requests upstream with optional response caching, hop-by-hop header stripping, response size limits and request fingerprinting.
- **[`serbewr/dabluvee-es`](https://psyb0t.github.io/aichteeteapee/websocket/)**, pronounced "WS", because why stop at one wordplay. Hub -> Client -> Connection, typed JSON events, handler registration, broadcast, and a Unix socket bridge for when you need to plug in external tools.
- **[`klhayyent`](https://psyb0t.github.io/aichteeteapee/client/)**, pronounced "client". Apparently that one needed saying too. JSON, raw, text and form bodies, bounded responses, request ID propagation, same-origin redirects, a real cookie jar, and the same `ErrorResponse` envelope as the server.

## Security

Secure by default. CORS blocks unknown origins, WebSocket validates `Origin` against `Host`, the server listens on loopback, file uploads strip directories from filenames and never overwrite, request IDs are validated, proxy and client responses are size-limited, and sensitive headers are filtered from echo responses.

Need to skip all that during local dev? One call:

```go
aichteeteapee.FuckSecurity()
```

CORS allows all origins, WebSocket accepts any origin. Call `aichteeteapee.UnfuckSecurity()` to go back. The [security page](https://psyb0t.github.io/aichteeteapee/security/) has the full list and the per-component overrides.

## Documentation

The full docs live at **[psyb0t.github.io/aichteeteapee](https://psyb0t.github.io/aichteeteapee/)**.

| Page | What's in it |
| --- | --- |
| [Server](https://psyb0t.github.io/aichteeteapee/server/) | Routers and groups, lifecycle, health, echo and upload handlers, static files, TLS, every env var |
| [Middleware](https://psyb0t.github.io/aichteeteapee/middleware/) | Every middleware with its options and defaults |
| [WebSockets](https://psyb0t.github.io/aichteeteapee/websocket/) | Events, hub, clients, connections, upgrade options, the Unix socket bridge |
| [Proxy](https://psyb0t.github.io/aichteeteapee/proxy/) | Forwarding, caching rules, fingerprinting |
| [Client](https://psyb0t.github.io/aichteeteapee/client/) | Requests, bodies, errors and status sentinels, cookies, redirects, options |
| [Errors](https://psyb0t.github.io/aichteeteapee/errors/) | The error envelope, every error code, the sentinels |
| [Constants and helpers](https://psyb0t.github.io/aichteeteapee/constants/) | Headers, content types, context keys, log fields, defaults |
| [Security](https://psyb0t.github.io/aichteeteapee/security/) | What's locked down, dev mode, what's on you |
| [Logging](https://psyb0t.github.io/aichteeteapee/logging/) | ctxscope, request fields on every line |
| [Development](https://psyb0t.github.io/aichteeteapee/development/) | Make targets, dev image, CI, releases |

The Go API reference is on [pkg.go.dev](https://pkg.go.dev/github.com/psyb0t/aichteeteapee), and the changes per release are in [CHANGELOG.md](https://github.com/psyb0t/aichteeteapee/blob/main/CHANGELOG.md).

## Development

Every target runs inside a pinned dev image (Go 1.26.9 plus the linter and scanners), so a local run and CI use the exact same toolchain. Docker and Make are all you need.

```bash
make lint           # shellcheck, shfmt, go fix, golangci-lint (strict)
make test           # go test -race ./...
make test-coverage  # coverage with 90% minimum
make sec            # gitleaks + govulncheck + semgrep; gates on findings
make docs           # build the docs site into .testing/docs-site
```

The rest is on the [development page](https://psyb0t.github.io/aichteeteapee/development/).

## Sponsor

<a href="https://www.buymeacoffee.com/psyb0t"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-blue.png" alt="Buy Me a Coffee" height="60" width="217"></a>

## License

MIT. See [LICENSE](https://github.com/psyb0t/aichteeteapee/blob/main/LICENSE).
