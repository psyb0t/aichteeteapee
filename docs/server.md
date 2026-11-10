---
title: "Server"
description: "The serbewr package: routers and groups on Go's ServeMux, health, echo and upload handlers, static files, TLS, shutdown and env configuration."
nav_order: 2
---

# Server

Pronounced "server". D'oooh you kno.

[`serbewr`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr) is built on `net/http` and routes with Go 1.22+ `ServeMux` patterns, so `/users/{id}` and `r.PathValue("id")` work the way the standard library says.

```go
srv, err := serbewr.New()                        // config from env vars, slog.Default()
srv, err := serbewr.NewWithLogger(logger)        // config from env vars, your logger
srv, err := serbewr.NewWithConfig(config)        // explicit config
srv, err := serbewr.NewWithConfigAndLogger(config, logger)
```

`New` and `NewWithLogger` fail only when the env vars do not parse. The logger is for the server's own lifecycle messages; request code logs through the context, see [Logging](logging.md).

## Router

A `Router` holds global middleware, static routes, and groups that nest as deep as you like. Each group adds its path prefix and its middleware to everything below it.

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
                {Method: http.MethodGet, Path: "/users/{id}", Handler: getUser},
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

err := srv.Start(ctx, router)
```

Each route registers the pattern `METHOD /prefix/path` on the mux, wrapped in the global middleware, then each enclosing group's middleware, in order. A route with an empty `Method` matches every method. A route with a nil `Handler` is skipped with a warning.

### Registering on the root group

You can skip the `Router` struct and register on the root group directly:

```go
root := srv.GetRootGroup()
root.GET("/health", srv.HealthHandler)
root.POST("/upload", srv.FileUploadHandler("./uploads"))

api := root.Group("/api/v1", authMiddleware)
api.GET("/users", listUsers)
api.POST("/users", createUser, auditMiddleware) // per-route middleware

err := srv.Start(ctx, nil)
```

`Group` has `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `Handle`, `HandleFunc` and `Use`. Pass `nil` to `Start` when you build routes this way. Passing a `Router` that has groups or static routes after you touched the root group panics, and its global middleware would not reach the routes you registered earlier.

## Lifecycle

`Start(ctx, router)` registers the routes, checks the TLS settings, opens the listeners and blocks. It returns:

- an error wrapping `ctx.Err()` when the context ends;
- `nil` when something calls `Stop`;
- the server error if a listener fails.

Either way it runs `Stop` on the way out. `Stop(ctx)` runs once: it shuts the HTTP and HTTPS servers down in parallel, gives in-flight requests up to `ShutdownTimeout`, closes the listeners and returns the first error.

`GetHTTPListenerAddr()` and `GetHTTPSListenerAddr()` return the bound addresses once the server is listening, which is how you find the port after listening on `127.0.0.1:0`. `GetListenerAddr()` is the deprecated name for the HTTP one. `GetMux()` and `GetLogger()` hand back the underlying `*http.ServeMux` and logger.

## Built-in handlers

### HealthHandler

Answers `200` with `{"status": "ok", "timestamp": "<UTC time>"}`.

### EchoHandler

Echoes back the method, path, query, headers and JSON body, which is handy for debugging a proxy or a client. It drops `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key` and `Proxy-Authorization` from the echoed headers. A request with a non-empty body that is not `application/json` gets `415 UNSUPPORTED_CONTENT_TYPE`. Behind [BasicAuth](middleware.md#basicauth), the response also has a `user` field with the authenticated name.

### FileUploadHandler

`srv.FileUploadHandler(dir, options...)` returns a handler that saves the multipart field named `file` into `dir`. It creates `dir` with mode `0750` when you build the handler.

```go
srv.FileUploadHandler("./uploads",
    serbewr.WithFilenamePrependType(serbewr.FilenamePrependTypeDateTime),
    serbewr.WithFileUploadMaxSize(50<<20), // 50MB total request limit
    serbewr.WithFileUploadHandlerPostprocessor(
        func(resp map[string]any, r *http.Request) (map[string]any, error) {
            resp["custom_field"] = "value"

            return resp, nil
        },
    ),
)
```

The saved name gets a prefix picked by `WithFilenamePrependType`:

| Option | Saved name |
| --- | --- |
| `FilenamePrependTypeUUID` (default) | `<uuid4>_<name>` |
| `FilenamePrependTypeDateTime` | `2006_01_02_15_04_05_<name>` |
| `FilenamePrependTypeNone` | `<name>` |

On success it answers `200`:

```json
{
  "status": "success",
  "original_filename": "report.pdf",
  "saved_filename": "6f1c2a9e-0b7d-4a63-9d1e-2f4b8c7a1e55_report.pdf",
  "size": 48213,
  "path": "6f1c2a9e-0b7d-4a63-9d1e-2f4b8c7a1e55_report.pdf"
}
```

A postprocessor can replace that map. If it returns an error, the client gets `500 INTERNAL_SERVER_ERROR`.

| Problem | Response |
| --- | --- |
| Method is not `POST` | `405 METHOD_NOT_ALLOWED` |
| Body is not a valid multipart form, or is bigger than the size limit | `400 INVALID_MULTIPART_FORM` |
| No `file` field | `400 NO_FILE_PROVIDED` |
| The file cannot be created or written, including when the name already exists | `500 FILE_SAVE_FAILED` |

The handler strips any directory from the client's file name with `filepath.Base`, so `../../etc/passwd` lands as `passwd` inside `dir`. Files are created with mode `0600` and `O_EXCL`, so an existing file is never overwritten. The whole request is capped at 100MB unless you set `WithFileUploadMaxSize`; a non-positive value keeps the default. `HTTP_SERVER_FILEUPLOADMAXMEMORY` is a different knob: how much of the form is held in memory before larger parts spill to temporary files, which are removed after the request.

## Static files

```go
Static: []serbewr.StaticRouteConfig{
    {
        Dir:                   "./public",
        Path:                  "/static",
        DirectoryIndexingType: serbewr.DirectoryIndexingTypeHTML,
    },
}
```

Each entry registers `GET <Path>/{path...}` on the root group, so it gets the global middleware and also answers `HEAD`. Entries with an empty `Dir` or `Path` are ignored.

| Request | Response |
| --- | --- |
| A file | The file, through `http.ServeFile`. |
| A directory with `index.html` | That `index.html`. |
| A directory without one, `DirectoryIndexingTypeNone` (default) | `403 DIRECTORY_LISTING_NOT_SUPPORTED` |
| A directory without one, `DirectoryIndexingTypeHTML` | An HTML listing, directories first, then files, both alphabetical. |
| A directory without one, `DirectoryIndexingTypeJSON` | A JSON array of `{"name", "isDir", "size", "modTime", "url"}` objects, same order. |
| A path with `..`, or a symlink that leads outside `Dir` | `403 PATH_TRAVERSAL_DENIED` |
| Nothing there | `404 FILE_NOT_FOUND` |

The handler caches whether a path exists and whether it is a directory, for 30 seconds and up to 1000 entries, dropping the oldest entry when full. That keeps a flood of requests for missing files off the disk. The flip side: a file you create right after someone asked for it can answer 404 for up to 30 seconds.

## TLS

```bash
export HTTP_SERVER_TLSENABLED=true
export HTTP_SERVER_TLSCERTFILE=/path/to/cert.pem
export HTTP_SERVER_TLSKEYFILE=/path/to/key.pem
```

With TLS on, the server listens for HTTP on `HTTP_SERVER_LISTENADDRESS` and for HTTPS on `HTTP_SERVER_TLSLISTENADDRESS` at the same time, both serving the same routes. `Start` refuses to run when TLS is on and the cert or key path is empty (`ErrTLSCertFileNotSpecified`, `ErrTLSKeyFileNotSpecified`) or the file does not exist.

## Configuration

`New` and `NewWithLogger` read these env vars. `NewWithConfig` takes a `serbewr.Config` with the same fields instead, and does not read the environment at all, so set every field you need. Durations use Go syntax (`15s`, `2m`), sizes are bytes.

| Variable | `Config` field | Default | What it sets |
| --- | --- | --- | --- |
| `HTTP_SERVER_LISTENADDRESS` | `ListenAddress` | `127.0.0.1:8080` | HTTP listen address. |
| `HTTP_SERVER_READTIMEOUT` | `ReadTimeout` | `15s` | Max time to read a whole request. |
| `HTTP_SERVER_READHEADERTIMEOUT` | `ReadHeaderTimeout` | `10s` | Max time to read the headers. |
| `HTTP_SERVER_WRITETIMEOUT` | `WriteTimeout` | `30s` | Max time to write the response. |
| `HTTP_SERVER_IDLETIMEOUT` | `IdleTimeout` | `60s` | Keep-alive idle timeout. |
| `HTTP_SERVER_SHUTDOWNTIMEOUT` | `ShutdownTimeout` | `10s` | How long `Stop` waits for in-flight requests. |
| `HTTP_SERVER_MAXHEADERBYTES` | `MaxHeaderBytes` | `1048576` (1MB) | Max request header size. |
| `HTTP_SERVER_FILEUPLOADMAXMEMORY` | `FileUploadMaxMemory` | `33554432` (32MB) | Multipart memory before parts spill to temp files. |
| `HTTP_SERVER_SERVICENAME` | `ServiceName` | `http-server` | Service name. Parsed and stored; nothing in the library reads it yet. |
| `HTTP_SERVER_TLSENABLED` | `TLSEnabled` | `false` | Start the HTTPS listener too. |
| `HTTP_SERVER_TLSLISTENADDRESS` | `TLSListenAddress` | `127.0.0.1:8443` | HTTPS listen address. |
| `HTTP_SERVER_TLSCERTFILE` | `TLSCertFile` | empty | TLS certificate path. |
| `HTTP_SERVER_TLSKEYFILE` | `TLSKeyFile` | empty | TLS private key path. |

Both default listen addresses are loopback. Set `HTTP_SERVER_LISTENADDRESS=0.0.0.0:8080` (or `:8080`) to accept connections from other hosts, for example inside a container.
