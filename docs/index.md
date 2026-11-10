---
title: "Home"
description: "aichteeteapee is a Go HTTP library on net/http: grouped routes, middleware, uploads, static files, WebSocket hubs, a caching proxy and a client."
nav_order: 1
permalink: /
seo:
  type: WebSite
---

# aichteeteapee

Pronounced "HTTP". The name is the whole joke. Moving on.

A Go HTTP library that does everything you need and nothing you don't. Spin up a server with middleware, WebSocket hubs, file uploads, static serving, and request proxying with caching, then call other services with a client that speaks the same error format. Secure defaults, without the boilerplate.

## Install

```bash
go get github.com/psyb0t/aichteeteapee
```

The module declares `go 1.26.9` in its [`go.mod`](https://github.com/psyb0t/aichteeteapee/blob/main/go.mod), so you need that Go version or newer.

## Quick look

<!-- {% raw %} -->
```go
package main

import (
    "context"
    "net/http"

    "github.com/psyb0t/aichteeteapee"
    "github.com/psyb0t/aichteeteapee/serbewr"
    "github.com/psyb0t/aichteeteapee/serbewr/middleware"
)

func main() {
    srv, err := serbewr.New()
    if err != nil {
        panic(err)
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
                    aichteeteapee.WriteJSON(
                        w,
                        http.StatusOK,
                        map[string]string{"msg": "hi"},
                    )
                },
            }},
        }},
    }

    if err := srv.Start(context.Background(), router); err != nil {
        panic(err)
    }
}
```
<!-- {% endraw %} -->

That listens on `127.0.0.1:8080` and answers `GET /hello` with `{"msg": "hi"}`. Every setting has a default and an `HTTP_SERVER_*` env var; [Server](server.md#configuration) lists them.

## The packages

Every package name is how you'd say it out loud. Yes, all of them.

| Package | Pronounced | What it does |
| --- | --- | --- |
| [`aichteeteapee`](https://pkg.go.dev/github.com/psyb0t/aichteeteapee) | "HTTP" | Header names, content types, error codes and envelopes, sentinels, defaults, request and response helpers. |
| [`serbewr`](server.md) | "server" | The server: routing groups, built-in handlers, static files, TLS, graceful shutdown. |
| [`serbewr/middleware`](middleware.md) | | RequestID, Logger, Recovery, BasicAuth, BearerAuth, CORS, SecurityHeaders, Timeout, EnforceRequestContentType. |
| [`serbewr/prawxxey`](proxy.md) | "proxy" | Forward a request upstream, with optional response caching. |
| [`serbewr/dabluvee-es`](websocket.md) | "WS" | WebSocket events, the `wshub` hub and the `wsunixbridge` Unix socket bridge. |
| [`klhayyent`](client.md) | "client" | Outbound HTTP client with bounded responses, same-origin redirects and a cookie jar. |

## Where to go next

| You want to | Read |
| --- | --- |
| Set up routes, groups, uploads, static files and TLS | [Server](server.md) |
| Add request IDs, logging, auth, CORS or timeouts | [Middleware](middleware.md) |
| Push events to browsers over WebSockets | [WebSockets](websocket.md) |
| Forward requests to another service and cache the answers | [Proxy](proxy.md) |
| Call another HTTP API | [Client](client.md) |
| Know every error code and what status it maps to | [Errors](errors.md) |
| Find the header, content type and default constants | [Constants and helpers](constants.md) |
| See what is locked down by default and how to open it up | [Security](security.md) |
| Get request IDs on every log line | [Logging](logging.md) |
| Build, lint and test the library itself | [Development](development.md) |

The Go package docs are on [pkg.go.dev](https://pkg.go.dev/github.com/psyb0t/aichteeteapee), and the source is on [GitHub](https://github.com/psyb0t/aichteeteapee).
