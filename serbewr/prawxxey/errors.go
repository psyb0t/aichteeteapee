package prawxxey

import "errors"

var (
	errNilContext           = errors.New("context is nil")
	errNilHTTPClient        = errors.New("HTTP client is nil")
	errNilRequestPayload    = errors.New("request payload is nil")
	errInvalidUpstreamURL   = errors.New("invalid upstream URL")
	errInvalidResponseLimit = errors.New("invalid maximum response body size")
	errResponseBodyTooLarge = errors.New(
		"upstream response body exceeds maximum size",
	)
)
