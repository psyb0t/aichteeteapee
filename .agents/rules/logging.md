# Logging

Prefix: LOG.

- **LOG1:** Code on a request path logs through `ctxscope.GetLogger(ctx)` with the request's context, so the attributes RequestID and Logger put on the scope (`requestId`, `method`, `path`, `ip`) reach every line. Do not pass a `*slog.Logger` around and do not pin one to the context. The server's own lifecycle messages use the `*slog.Logger` given to `NewWithLogger` (`s.logger`). `wshub`, `wsunixbridge` setup and `dabluveees.NewEvent` run outside a request and use `slog` directly.
- **LOG2:** A field key that appears in more than one place is a constant in `log_fields.go` (`FieldRequestID`, `FieldClientID`, `FieldHubName` and the rest). Use the constant, or add one.
- **LOG3:** Warnings that explain a rejected request carry a stable `reason` value in snake_case (`invalid_multipart_form`, `missing_upload_file`, `cache_read_failed`), so a log query can count them.
- **LOG4:** Never log a password, token, cookie, request body, upstream URL or query string. `EchoHandler` drops `Authorization`, `Cookie`, `Set-Cookie`, `X-Api-Key` and `Proxy-Authorization` from its echo. `prawxxey` and `klhayyent` log method, status, duration and size only, and log `aichteeteapee.ErrAPIError` in place of the transport error. Keep it that way. BasicAuth logs the attempted user name on failure and nothing else.
- **LOG5:** The Logger middleware includes the raw query by default (`IncludeQuery: true`). A change to what Logger records needs a note in `docs/middleware.md` and a reason that rules out secrets.
