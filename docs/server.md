# serbewr — HTTP server

Pronounced "server". D'oooh you kno.

Built on `net/http` with routing via Go 1.22+ `ServeMux` patterns.

```go
srv, _ := serbewr.New()                    // defaults from env vars
srv, _ := serbewr.NewWithConfig(config)     // explicit config
```

## Router

Groups, nested groups, per-group middleware, method-based routing.

```go
router := &serbewr.Router{
    GlobalMiddlewares: []middleware.Middleware{
        middleware.RequestID(),
        middleware.Logger(),
        middleware.Recovery(),
        middleware.SecurityHeaders(),
        middleware.CORS(),
    },
    Static: []serbewr.StaticRouteConfig{
        {Dir: "./static", Path: "/static"},
    },
    Groups: []serbewr.GroupConfig{
        {
            Path: "/api/v1",
            Routes: []serbewr.RouteConfig{
                {Method: http.MethodGet, Path: "/users", Handler: listUsers},
                {Method: http.MethodPost, Path: "/users", Handler: createUser},
            },
        },
        {
            Path: "/admin",
            Middlewares: []middleware.Middleware{
                middleware.BasicAuth(middleware.WithBasicAuthUsers(users)),
            },
            Routes: []serbewr.RouteConfig{
                {Method: http.MethodGet, Path: "/stats", Handler: adminStats},
            },
        },
    },
}

srv.Start(ctx, router)
```

You can also use the root group directly:

```go
root := srv.GetRootGroup()
root.GET("/health", srv.HealthHandler)
root.POST("/upload", srv.FileUploadHandler("./uploads"))

api := root.Group("/api/v1", authMiddleware)
api.GET("/users", listUsers)
api.POST("/users", createUser)
```

## Built-in handlers

**HealthHandler** — returns `{"status": "ok", "timestamp": "..."}`.

**EchoHandler** — echoes back method, path, query, headers (sensitive headers filtered), and body. Useful for debugging.

**FileUploadHandler** — multipart file upload with configurable filename strategy:

```go
srv.FileUploadHandler("./uploads",
    serbewr.WithFilenamePrependType(serbewr.FilenamePrependTypeUUID),     // default
    serbewr.WithFilenamePrependType(serbewr.FilenamePrependTypeDateTime), // 2024_01_15_...
    serbewr.WithFilenamePrependType(serbewr.FilenamePrependTypeNone),     // original name
    serbewr.WithFileUploadHandlerPostprocessor(func(resp map[string]any, r *http.Request) (map[string]any, error) {
        resp["custom_field"] = "value"
        return resp, nil
    }),
)
```

Filenames are sanitized (`filepath.Base`) to prevent path traversal. Files are created with `0600` permissions and `O_EXCL` (won't overwrite existing files).

## Static file serving

Path traversal protection (both `..` components and symlink resolution), file existence caching with LRU eviction, and optional directory indexing.

```go
Static: []serbewr.StaticRouteConfig{
    {
        Dir:                   "./public",
        Path:                  "/static",
        DirectoryIndexingType: serbewr.DirectoryIndexingTypeHTML, // or JSON, or None
    },
}
```

Directories with an `index.html` serve it automatically. Without one, behavior depends on `DirectoryIndexingType`.

## TLS

```bash
export HTTP_SERVER_TLSENABLED=true
export HTTP_SERVER_TLSCERTFILE=/path/to/cert.pem
export HTTP_SERVER_TLSKEYFILE=/path/to/key.pem
```

HTTP and HTTPS servers run simultaneously on separate ports.

## Configuration

All config is read from environment variables with sensible defaults. Or just use `NewWithConfig(config)` and set everything in code — your call.

| Variable | Default | Description |
|---|---|---|
| `HTTP_SERVER_LISTENADDRESS` | `127.0.0.1:8080` | HTTP listen address |
| `HTTP_SERVER_READTIMEOUT` | `15s` | Max time to read request |
| `HTTP_SERVER_READHEADERTIMEOUT` | `10s` | Max time to read headers |
| `HTTP_SERVER_WRITETIMEOUT` | `30s` | Max time to write response |
| `HTTP_SERVER_IDLETIMEOUT` | `60s` | Keep-alive idle timeout |
| `HTTP_SERVER_SHUTDOWNTIMEOUT` | `10s` | Graceful shutdown timeout |
| `HTTP_SERVER_MAXHEADERBYTES` | `1MB` | Max header size |
| `HTTP_SERVER_FILEUPLOADMAXMEMORY` | `32MB` | Max upload memory before temp file |
| `HTTP_SERVER_SERVICENAME` | `http-server` | Service name for logging |
| `HTTP_SERVER_TLSENABLED` | `false` | Enable HTTPS |
| `HTTP_SERVER_TLSLISTENADDRESS` | `127.0.0.1:8443` | HTTPS listen address |
| `HTTP_SERVER_TLSCERTFILE` | — | TLS certificate path |
| `HTTP_SERVER_TLSKEYFILE` | — | TLS private key path |
