---
title: "Logging"
description: "How aichteeteapee uses ctxscope so every log line in a request carries requestId, method, path and ip, and how to configure the slog handler once."
nav_order: 10
---

# Logging

All middleware, handler, proxy and client code logs through [ctxscope](https://github.com/psyb0t/ctxscope), which keeps log attributes on the request context.

The middleware chain builds up the request's scope progressively:

1. **RequestID** puts `requestId` on the scope.
2. **Logger** adds `method`, `path` and `ip`.
3. Downstream code calls `ctxscope.GetLogger(ctx)` and gets all of them automatically.

```go
func getUser(w http.ResponseWriter, r *http.Request) {
    logger := ctxscope.GetLogger(r.Context())
    logger.Info("loading user", "id", r.PathValue("id"))
    // the line carries requestId, method, path and ip next to id
}
```

The context carries attributes, not a logger. `ctxscope.GetLogger` applies them to `slog.Default()` at the moment you ask, so configure your handler once at startup with `slog.SetDefault` and every line picks it up:

```go
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
```

Because they are data rather than a logger, the same attributes also cross a process boundary via `ctxscope.ToJSON` / `ctxscope.FromJSON`.

Put RequestID before Logger in the chain. Logger writes its line from the context it built, so an ID added further in never shows up on it.

Field names come from the constants in [`log_fields.go`](https://github.com/psyb0t/aichteeteapee/blob/main/log_fields.go): `FieldRequestID`, `FieldMethod`, `FieldPath`, `FieldIP` and friends. See [Constants and helpers](constants.md#log-field-names).

No explicit logger passing. Every log line from every middleware and handler automatically includes the full request context.

## Outside a request

A few things log without a request context:

- the server's start and shutdown messages go to the `*slog.Logger` you passed to `NewWithLogger` or `NewWithConfigAndLogger`, `slog.Default()` otherwise;
- the WebSocket hub, clients and connections, and `dabluveees.NewEvent`, log through `slog`'s package functions, which means `slog.Default()`;
- `WriteJSON` logs an encoding failure through `slog.Error`.

## What never gets logged

The library keeps secrets out of its own lines. The proxy and the client log the method, status, duration and body size, never the URL, query, headers or body. Auth failures log that they failed; BasicAuth adds the user name that was tried. The Logger middleware logs the raw query string unless you pass `WithIncludeQuery(false)`, and only the headers you name in `WithIncludeHeaders`.
