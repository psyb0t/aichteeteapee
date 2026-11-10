---
title: "Errors"
description: "The aichteeteapee error envelope, every ErrorCode with its status and prebuilt response, ErrorCodeFromHTTPStatus, and the sentinel errors."
nav_order: 7
---

# Errors

Every error this library answers with is the same JSON envelope, and [`klhayyent`](client.md) decodes the same envelope on the way back in. Use it in your own handlers and every service in the chain speaks one error format.

## The envelope

```go
type ErrorResponse struct {
    Code    string `json:"code,omitempty"`    // machine-readable, UPPER_SNAKE
    Message string `json:"message,omitempty"` // for humans
    Details any    `json:"details,omitempty"` // whatever context helps
}
```

```go
aichteeteapee.WriteJSON(w, http.StatusNotFound, aichteeteapee.ErrorResponseNotFound)
```

```json
{
  "code": "NOT_FOUND",
  "message": "Not found"
}
```

`WriteJSON(w, status, data)` sets `Content-Type: application/json`, writes the status and encodes `data` with two-space indentation. An encoding failure is logged, since the status line is already gone by then.

To add details, copy a prebuilt value and set the field, the way [EnforceRequestContentType](middleware.md#enforcerequestcontenttype) does:

```go
resp := aichteeteapee.ErrorResponseValidationFailed
resp.Details = map[string]string{"email": "required"}
aichteeteapee.WriteJSON(w, http.StatusUnprocessableEntity, resp)
```

## Error codes

All codes are `ErrorCode*` constants in [`error_codes.go`](https://github.com/psyb0t/aichteeteapee/blob/main/error_codes.go). The prebuilt `ErrorResponse*` values live in [`error_responses.go`](https://github.com/psyb0t/aichteeteapee/blob/main/error_responses.go).

### Status codes

`ErrorCodeFromHTTPStatus(status)` maps these statuses to their code. Any other status, 415 included, maps to `INTERNAL_SERVER_ERROR`.

| Status | Code | Prebuilt response | Message | Written by |
| --- | --- | --- | --- | --- |
| 400 | `BAD_REQUEST` | `ErrorResponseBadRequest` | Bad request | |
| 401 | `UNAUTHORIZED` | `ErrorResponseUnauthorized` | Unauthorized | BasicAuth, BearerAuth (message configurable) |
| 403 | `FORBIDDEN` | `ErrorResponseForbidden` | Access forbidden | |
| 404 | `NOT_FOUND` | `ErrorResponseNotFound` | Not found | |
| 405 | `METHOD_NOT_ALLOWED` | `ErrorResponseMethodNotAllowed` | Method not allowed | FileUploadHandler |
| 409 | `CONFLICT` | `ErrorResponseConflict` | Conflict | |
| 410 | `GONE` | `ErrorResponseGone` | Gone | |
| 413 | `PAYLOAD_TOO_LARGE` | `ErrorResponsePayloadTooLarge` | Payload too large | |
| 422 | `UNPROCESSABLE_ENTITY` | `ErrorResponseUnprocessableEntity` | Unprocessable entity | |
| 429 | `TOO_MANY_REQUESTS` | `ErrorResponseTooManyRequests` | Too many requests | |
| 500 | `INTERNAL_SERVER_ERROR` | `ErrorResponseInternalServerError` | Internal server error | Recovery, upload postprocessor failures, directory listing failures |
| 501 | `NOT_IMPLEMENTED` | `ErrorResponseNotImplemented` | Not implemented | |
| 502 | `BAD_GATEWAY` | `ErrorResponseBadGateway` | Bad gateway | |
| 503 | `SERVICE_UNAVAILABLE` | `ErrorResponseServiceUnavailable` | Service unavailable | |
| 504 | `GATEWAY_TIMEOUT` | `ErrorResponseGatewayTimeout` | Gateway timeout | Timeout (with its own longer message) |

### Library and application codes

These have no status mapping. The status in the table is the one the library uses when it writes them.

| Code | Prebuilt response | Message | Written by |
| --- | --- | --- | --- |
| `VALIDATION_FAILED` | `ErrorResponseValidationFailed` | Validation failed | |
| `RATE_LIMITED` | none | | |
| `ENDPOINT_NOT_FOUND` | `ErrorResponseEndpointNotFound` | Endpoint not found | |
| `FILE_NOT_FOUND` | `ErrorResponseFileNotFound` | File not found | Static files, 404 |
| `DIRECTORY_LISTING_NOT_SUPPORTED` | `ErrorResponseDirectoryListingNotSupported` | Directory listing is not supported | Static files, 403 |
| `PATH_TRAVERSAL_DENIED` | `ErrorResponsePathTraversalDenied` | Path traversal denied | Static files, 403 |
| `MISSING_USER_ID` | `ErrorResponseMissingUserID` | User ID is required | |
| `INVALID_USER_ID` | `ErrorResponseInvalidUserID` | Invalid user ID format | |
| `MISSING_CONTENT_TYPE` | `ErrorResponseMissingContentType` | Content-Type header is required | EnforceRequestContentType, 400 |
| `UNSUPPORTED_CONTENT_TYPE` | `ErrorResponseUnsupportedContentType` | Unsupported content type | EnforceRequestContentType and EchoHandler, 415 |
| `INVALID_MULTIPART_FORM` | `ErrorResponseInvalidMultipartForm` | Invalid multipart form | FileUploadHandler, 400 |
| `NO_FILE_PROVIDED` | `ErrorResponseNoFileProvided` | No file provided | FileUploadHandler, 400 |
| `FILE_SAVE_FAILED` | `ErrorResponseFileSaveFailed` | Failed to save file | FileUploadHandler, 500 |

`ErrorCode` is an alias for `string`, so your own codes fit in the same field.

## Sentinel errors

[`errors.go`](https://github.com/psyb0t/aichteeteapee/blob/main/errors.go) declares plain `errors.New` sentinels to match with `errors.Is`. The [client](client.md#errors-and-limits) returns the status ones from `HTTPError.Unwrap`, and the server returns the TLS ones from `Start`.

| Group | Sentinels |
| --- | --- |
| 4xx | `ErrBadRequest`, `ErrUnauthorized`, `ErrNotAuthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrMethodNotAllowed`, `ErrConflict`, `ErrGone`, `ErrUnprocessableEntity`, `ErrTooManyRequests` |
| 5xx | `ErrInternalServer`, `ErrBadGateway`, `ErrServiceUnavailable`, `ErrGatewayTimeout` |
| Responses | `ErrInvalidResponse`, `ErrEmptyResponse`, `ErrUnexpectedResponseStatus` |
| Anti-bot | `ErrBotDetected`, `ErrChallengeBlocked`, `ErrCaptchaRequired` |
| API clients | `ErrAPIError`, `ErrAPIKeyNotSet`, `ErrNilRequestBody` |
| TLS | `ErrTLSCertFileNotSpecified`, `ErrTLSKeyFileNotSpecified` |

The client package adds its own: `ErrInvalidBaseURL`, `ErrInvalidRequestTarget`, `ErrInvalidOption` and `ErrResponseBodyTooLarge`.
