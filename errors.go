package aichteeteapee

import "errors"

// 4xx Client Errors.
var (
	ErrBadRequest          = errors.New("bad request")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrNotAuthenticated    = errors.New("not authenticated")
	ErrForbidden           = errors.New("forbidden")
	ErrNotFound            = errors.New("not found")
	ErrMethodNotAllowed    = errors.New("method not allowed")
	ErrConflict            = errors.New("conflict")
	ErrGone                = errors.New("gone")
	ErrUnprocessableEntity = errors.New("unprocessable entity")
	ErrTooManyRequests     = errors.New("too many requests")
)

// 5xx Server Errors.
var (
	ErrInternalServer     = errors.New("internal server error")
	ErrBadGateway         = errors.New("bad gateway")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrGatewayTimeout     = errors.New("gateway timeout")
)

// Response handling errors.
var (
	ErrInvalidResponse          = errors.New("invalid response")
	ErrEmptyResponse            = errors.New("empty response")
	ErrUnexpectedResponseStatus = errors.New("unexpected response status")
)

// Anti-bot / scraping errors.
var (
	ErrBotDetected      = errors.New("bot detected")
	ErrChallengeBlocked = errors.New("challenge blocked")
	ErrCaptchaRequired  = errors.New("captcha required")
)
