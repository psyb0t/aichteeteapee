---
title: "MCP"
description: "Mount a stateless MCP server with Streamable HTTP, use JWT bearer auth with WorkOS Connect, and keep event streams working through a proxy."
nav_order: 12
---

# MCP

`serbewr/em-see-pee` (package `emseepee`) mounts the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) under a [server route group](server.md#router). Register tools on the SDK's `mcp.Server`, then put the resulting route in `serbewr.Router.Groups` and pass the router to `Server.Start`.

The endpoint uses stateless Streamable HTTP. POST handles MCP messages and can stream responses as SSE. Send `Content-Type: application/json` and `Accept: application/json, text/event-stream` on POST. Stateless mode has no session ID, standalone GET stream or server-initiated requests. The SDK returns 405 on GET and DELETE after any group middleware runs.

## WorkOS Connect

Use `serbewr/em-see-pee/auth` to protect an MCP endpoint with OAuth. It takes the public endpoint URL, the issuer, and a token verifier. The same helper works with any provider that issues tokens for the endpoint and supports the client's OAuth flow.

If your installed release predates `serbewr/em-see-pee`, use the development version with `go get github.com/psyb0t/aichteeteapee@main`.

In WorkOS Connect, enable CIMD and register the exact public MCP endpoint URL as a Resource Indicator. Use the `issuer` and `jwks_uri` values from OAuth discovery as trusted app configuration. Share a verifier between routes served by the same issuer and set the audience to the full public URL of each MCP endpoint. If a client omits the `resource` parameter, WorkOS can issue a token for the environment's client ID unless a matching default Resource Indicator is configured. [WorkOS MCP documentation](https://workos.com/docs/authkit/mcp)

```go
import (
    "net/http"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/psyb0t/aichteeteapee/serbewr"
    emseepee "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee"
    mcpauth "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee/auth"
    "github.com/psyb0t/aichteeteapee/serbewr/middleware"
)

func imageRouter(tools *mcp.Server) (*serbewr.Router, error) {
    const issuer = "https://example.authkit.app"
    const jwksURL = "https://example.authkit.app/oauth2/jwks"
    const origin = "https://example.org"
    const path = "/v1/apps/image-tools/mcp"

    verifier, err := middleware.NewJWKSVerifier(issuer, jwksURL)
    if err != nil {
        return nil, err
    }

    protection, err := mcpauth.New(
        origin+path,
        issuer,
        verifier.Verify,
    )
    if err != nil {
        return nil, err
    }

    route, err := emseepee.NewRoute(
        path,
        func(*http.Request) *mcp.Server { return tools },
        emseepee.WithPublicOrigin(origin),
        emseepee.WithWriteTimeout(time.Minute),
    )
    if err != nil {
        return nil, err
    }

    return &serbewr.Router{
        GlobalMiddlewares: []middleware.Middleware{
            middleware.RequestID(), middleware.Logger(), middleware.Recovery(),
        },
        Groups: []serbewr.GroupConfig{
            {
                Path:        "/",
                Middlewares: []middleware.Middleware{protection.Middleware},
                Routes:      []serbewr.RouteConfig{route},
            },
            {
                Path:   "/",
                Routes: []serbewr.RouteConfig{protection.MetadataRoute},
            },
        },
    }, nil
}
```

The helper derives the public discovery route from the resource URL. Mount it at the router root, outside the protected group, and preserve the configured MCP path through the proxy. Missing or invalid credentials get a JSON `401` with a `WWW-Authenticate` challenge pointing at discovery. To require scopes on every request, pass `mcpauth.WithRequiredScopes("images:read")` and configure that scope at your provider. A valid token missing a required scope gets `403` with an `insufficient_scope` challenge.

See [JWTBearerAuth](middleware.md#jwtbearerauth) for token requirements, cache behavior and verifier options.

In your tool handler, call `middleware.PrincipalFromContext(ctx)`. Check tool permissions, account state and ownership there. For organization-specific resources, check that the principal's `OrganizationID` matches the organization in the route. A user-only app should reject the `ai_agent` subject profile. An issued token can remain valid until expiry after grant revocation. Check your local access policy when disconnection must take effect immediately. [WorkOS token verification and revocation](https://workos.com/docs/authkit/mcp#token-verification)

## Origins and hosts

Every request runs these checks before the SDK sees it, in this order. Each refusal writes the [error envelope](errors.md) and logs a warning with a stable `reason`.

| Check | Default | Refusal |
| --- | --- | --- |
| `WithAllowedHosts(hosts...)` | off | `421 MISDIRECTED_REQUEST`, `reason=invalid_host` |
| Browser `Origin` | the request's own scheme and `Host`, or `WithPublicOrigin` | `403 FORBIDDEN`, `reason=invalid_origin` |
| Cross-origin protection | on with `WithAllowedOrigins` | `403 FORBIDDEN`, `reason=cross_origin_rejected` |

A request with no `Origin` header passes the origin check, because only browsers send one. A request with several `Origin` headers is refused.

`WithAllowedOrigins(origins...)` replaces the same-origin check with an allowlist. Each entry is a bare `http` or `https` origin, `scheme://host[:port]` with no path, query or user info, and `NewRoute` returns `ErrBadRequest` for anything else. The request's `Origin` must match one entry, compared with a lowercase scheme and host. The port is part of the origin, so `http://localhost` and `http://localhost:8080` are different entries. `WithPublicOrigin` joins the same list. Calling `WithAllowedOrigins()` with no origins lets non-browser clients and the public origin through and refuses every other browser. With an allowlist set, the route also wraps the SDK in net/http `CrossOriginProtection` that trusts the listed origins, so a browser request marked `Sec-Fetch-Site: cross-site` without an allowed `Origin` is refused.

`WithAllowedHosts(hosts...)` is the DNS rebinding guard. Entries are DNS names or IP addresses without a port, with IPv6 optionally in brackets (`localhost`, `127.0.0.1`, `[::1]`, `mcp.example.org`). The request `Host` must name one of them, compared without case, on any valid port. An empty list, an entry with a port or zone, or an invalid name is a configuration error. The SDK has its own check that refuses a non-loopback `Host` on a loopback listener. When an entry is not a loopback host, the route turns that check off with `DisableLocalhostProtection`, because the allowlist now decides.

```go
route, err := emseepee.NewRoute(
    "/mcp",
    func(*http.Request) *mcp.Server { return tools },
    emseepee.WithAllowedHosts("localhost", "127.0.0.1", "[::1]", "mcp.example.org"),
    emseepee.WithAllowedOrigins("http://localhost:8080", "https://mcp.example.org"),
    emseepee.WithoutWriteDeadline(),
    emseepee.WithSDKOptions(mcp.StreamableHTTPOptions{MaxRequestBodyBytes: 16 << 20}),
)
```

## Deadlines and cancellation

A positive `WithWriteTimeout` overrides the listener write deadline and adds a request context deadline. The default of zero keeps the listener deadline and adds no route deadline. `WithoutWriteDeadline()` clears the listener deadline for the route, so a tool call can stream for hours; a response writer without deadline support is served unchanged. The two options conflict, and `NewRoute` returns `ErrBadRequest` when both are set. Apply request timeout middleware to REST groups so it doesn't end an MCP stream early.

A tool call's context ends when its HTTP request ends: the client disconnects, the route's write timeout passes, or the server shuts down. The SDK does that itself for the newest protocol version. For older versions it detaches the method context, so the route adds a receiving middleware that binds the method context to the HTTP request. It adds the middleware once to each `*mcp.Server` that `getServer` returns, on that server's first request; a server shared by two routes gets it once per route, which is harmless. Requests that reach the server some other way, such as stdio, are unaffected.

## Streaming through a proxy

Forward `Authorization` and preserve the validated public `Host`. Disable response buffering and caching for the MCP location. Set the proxy read timeout to accommodate the route's streams. `WithPublicOrigin` checks the request's `Origin` header when present and supports TLS termination at the edge. If the SDK's loopback Host check rejects the public host, list it in `WithAllowedHosts`, which also turns that check off. If you instead disable it with `WithSDKOptions(mcp.StreamableHTTPOptions{DisableLocalhostProtection: true})`, allowlist the public Host at the edge and restrict direct access to the listener.

`WithSDKOptions` passes SDK options through, for example to select JSON responses or adjust the default 4 MiB request body limit. The route always sets `Stateless` and `PropagateRequestCancellation`.
