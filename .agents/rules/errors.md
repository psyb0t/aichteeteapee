# Errors

Prefix: ERR.

- **ERR1:** Every HTTP error this library writes is the `aichteeteapee.ErrorResponse` envelope (`code`, `message`, optional `details`) written with `aichteeteapee.WriteJSON`. Use a prebuilt `ErrorResponse*` value from `error_responses.go` when one fits; copy it and set `Details` instead of building a new literal, as `EnforceRequestContentType` does.
- **ERR2:** Error codes are UPPER_SNAKE `ErrorCode*` constants in `error_codes.go`. They are part of the API: never rename one or reuse it for another meaning. A new HTTP status code mapping goes into `httpStatusToErrorCode`, with a case in `error_codes_test.go`, and the client's `statusError`/`serverStatusError` in `klhayyent/client.go` gains the matching sentinel when one exists.
- **ERR3:** Public sentinels callers match with `errors.Is` live in the root `errors.go`. Package-internal sentinels stay unexported in the package's `errors.go` (`serbewr/prawxxey/errors.go`), except where callers need them (`klhayyent/errors.go`).
- **ERR5:** Error text sent to a client never comes from an error chain. `ctxerrors` adds file, line and function to `Error()`, so the chain goes to the log and the client gets the fixed envelope message.
- **ERR6:** Errors that cross to an upstream or a dependency strip the `*url.Error` wrapper before wrapping (`sanitizeUpstreamError` in `prawxxey`, `sanitizeTransportError` in `klhayyent`), so a URL with tokens or query values never lands in an error string or a log.
