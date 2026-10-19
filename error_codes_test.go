package aichteeteapee

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeFromHTTPStatus(t *testing.T) {
	t.Parallel()

	testCases := map[int]ErrorCode{
		http.StatusBadRequest:            ErrorCodeBadRequest,
		http.StatusUnauthorized:          ErrorCodeUnauthorized,
		http.StatusForbidden:             ErrorCodeForbidden,
		http.StatusNotFound:              ErrorCodeNotFound,
		http.StatusMethodNotAllowed:      ErrorCodeMethodNotAllowed,
		http.StatusConflict:              ErrorCodeConflict,
		http.StatusGone:                  ErrorCodeGone,
		http.StatusRequestEntityTooLarge: ErrorCodePayloadTooLarge,
		http.StatusUnprocessableEntity:   ErrorCodeUnprocessableEntity,
		http.StatusTooManyRequests:       ErrorCodeTooManyRequests,
		http.StatusInternalServerError:   ErrorCodeInternalServerError,
		http.StatusNotImplemented:        ErrorCodeNotImplemented,
		http.StatusBadGateway:            ErrorCodeBadGateway,
		http.StatusServiceUnavailable:    ErrorCodeServiceUnavailable,
		http.StatusGatewayTimeout:        ErrorCodeGatewayTimeout,
		http.StatusTeapot:                ErrorCodeInternalServerError,
	}

	for status, expected := range testCases {
		assert.Equal(t, expected, ErrorCodeFromHTTPStatus(status))
	}
}
