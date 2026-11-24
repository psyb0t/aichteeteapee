---
title: "MCP"
description: "Mount a stateless MCP server with Streamable HTTP, use JWT bearer auth with WorkOS Connect, and keep event streams working through a proxy."
nav_order: 12
---

# MCP

`serbewr/em-see-pee` (package `emseepee`) mounts the official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) under a [server route group](server.md#router). Register tools on the SDK's `mcp.Server`, then put the resulting route in `serbewr.Router.Groups` and pass the router to `Server.Start`.

The endpoint uses stateless Streamable HTTP. POST handles MCP messages and can stream responses as SSE. Send `Content-Type: application/json` and `Accept: application/json, text/event-stream` on POST. Stateless mode has no session ID, standalone GET stream or server-initiated requests. The SDK returns 405 on GET and DELETE after any group middleware runs.

## WorkOS Connect

If your installed release predates `serbewr/em-see-pee`, use the development version with `go get github.com/psyb0t/aichteeteapee@main`.

In WorkOS Connect, enable CIMD and register the exact public MCP endpoint URL as a Resource Indicator. Use the `issuer` and `jwks_uri` values from OAuth discovery as trusted app configuration. Share a verifier between routes served by the same issuer and set the audience to the full public URL of each MCP endpoint. If a client omits the `resource` parameter, WorkOS can issue a token for the environment's client ID unless a matching default Resource Indicator is configured. [WorkOS MCP documentation](https://workos.com/docs/authkit/mcp)

```go
import (
    "net/http"
    "time"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/psyb0t/aichteeteapee"
    "github.com/psyb0t/aichteeteapee/serbewr"
    emseepee "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee"
    "github.com/psyb0t/aichteeteapee/serbewr/middleware"
)

func imageRouter(tools *mcp.Server) (*serbewr.Router, error) {
    const issuer = "https://example.authkit.app"
    const jwksURL = "https://example.authkit.app/oauth2/jwks"
    const origin = "https://example.org"
    const path = "/v1/apps/image-tools/mcp"
    const metadataPath = "/.well-known/oauth-protected-resource" + path

    verifier, err := middleware.NewJWKSVerifier(issuer, jwksURL)
    if err != nil {
        return nil, err
    }

    auth, err := middleware.JWTBearerAuth(
        verifier.Verify,
        origin+path,
        middleware.WithJWTBearerResourceMetadata(origin+metadataPath),
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
                Middlewares: []middleware.Middleware{auth},
                Routes:      []serbewr.RouteConfig{route},
            },
            {
                Path: "/",
                Routes: []serbewr.RouteConfig{{
                    Method: http.MethodGet,
                    Path:   metadataPath,
                    Handler: func(w http.ResponseWriter, _ *http.Request) {
                        aichteeteapee.WriteJSON(w, http.StatusOK, map[string]any{
                            "resource":                 origin + path,
                            "authorization_servers":    []string{issuer},
                            "bearer_methods_supported": []string{"header"},
                        })
                    },
                }},
            },
        },
    }, nil
}
```

The public metadata route gives clients the exact resource URL and the issuer. Start the server with the router returned by `imageRouter` and pass in the SDK server with your tools already registered. Missing or invalid credentials get HTTP 401 with the library's JSON error envelope and a `WWW-Authenticate` challenge pointing at the metadata. After authorization, MCP protocol responses follow the SDK's wire format.

See [JWTBearerAuth](middleware.md#jwtbearerauth) for token requirements, cache behavior and verifier options.

In your tool handler, call `middleware.BearerPrincipalFromContext(ctx)`. Check permissions, account state and ownership there. For organization-specific resources, check that the principal's `OrganizationID` matches the organization in the route. A user-only app should reject the `ai_agent` subject profile. An issued token can remain valid until expiry after grant revocation. Check your local access policy when disconnection must take effect immediately. [WorkOS token verification and revocation](https://workos.com/docs/authkit/mcp#token-verification)

## Streaming through a proxy

Forward `Authorization` and preserve the validated public `Host`. Disable response buffering and caching for the MCP location. Set the proxy read timeout to accommodate the route's streams. `WithPublicOrigin` checks the request's `Origin` header when present and supports TLS termination at the edge. If the SDK's loopback Host check rejects the public host, prefer a private upstream network. If you disable it with `WithSDKOptions(mcp.StreamableHTTPOptions{DisableLocalhostProtection: true})`, allowlist the public Host at the edge and restrict direct access to the listener.

A positive `WithWriteTimeout` overrides the listener write deadline and adds a request context deadline. The default of zero keeps the listener deadline and adds no route deadline. Apply request timeout middleware to REST groups so it doesn't end an MCP stream early. Bound tool work separately, especially for clients on protocol versions that lack HTTP cancellation propagation. Use `WithSDKOptions` to select JSON responses or adjust the default 4 MiB request body limit.
